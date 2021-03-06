package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx"
)

const stmtBank = "update_bank"

type DB struct {
	*pgx.ConnPool
	banktotals chan bankUpdate
}

type bankUpdate struct {
	Name string
	Bank int64
}

var db *DB

func connectDB() error {
	cfg, err := pgx.ParseEnvLibpq()
	if err != nil {
		return err
	}
	pool, err := pgx.NewConnPool(pgx.ConnPoolConfig{
		ConnConfig: cfg,
		AfterConnect: func(conn *pgx.Conn) error {
			_, err := conn.Prepare(stmtBank, "INSERT INTO banks (username, bank, best) VALUES ($1, $2, $2) ON CONFLICT (username) DO UPDATE SET bank = $2, best = greatest($2, banks.best), last = now()")
			return err
		},
	})
	if err != nil {
		return err
	}
	db = &DB{
		ConnPool:   pool,
		banktotals: make(chan bankUpdate, 1000),
	}
	go db.bankUpdater()
	return nil
}

func (db *DB) SetBank(username string, bank int64) {
	select {
	case db.banktotals <- bankUpdate{username, bank}:
	default:
	}
}

func (db *DB) AddHistory(username string, bank int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecEx(ctx, "INSERT INTO bank_history (username, bank) VALUES ($1, $2)", nil, username, bank)
	if err != nil {
		log.Printf("error: updating bank history: %s", err)
	}
}

func (db *DB) bankUpdater() {
	var banks []bankUpdate
	t := time.NewTimer(0)
	for {
		select {
		case <-t.C:
			if len(banks) == 0 {
				t.Reset(time.Hour)
				continue
			}
			if err := db.sendBatch(banks); err != nil {
				log.Printf("error: updating bank totals: %s", err)
			}
			banks = banks[:0]
		case item := <-db.banktotals:
			banks = append(banks, item)
			if len(banks) > 250 {
				if err := db.sendBatch(banks); err != nil {
					log.Printf("error: updating bank totals: %s", err)
				}
				banks = banks[:0]
				t.Reset(time.Hour)
			} else {
				t.Reset(time.Second)
			}
		}
	}
}

func (db *DB) sendBatch(banks []bankUpdate) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	b := db.BeginBatch()
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("error: batch update timed out")
		}
	}()
	for _, item := range banks {
		if item.Name == "" || item.Bank == 0 {
			continue
		}
		b.Queue(stmtBank, []interface{}{item.Name, item.Bank}, nil, nil)
	}
	if err := b.Send(ctx, nil); err != nil {
		b.Close()
		return err
	}
	if err := b.Close(); err != nil {
		return err
	}
	if d := time.Since(start); d > 100*time.Millisecond {
		log.Printf("warning: updating banks took %s", d)
	}
	return nil
}
