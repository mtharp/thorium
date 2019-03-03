package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/jackc/pgx"
	"golang.org/x/oauth2"
)

var db *pgx.ConnPool

func connectDB() error {
	cfg, err := pgx.ParseEnvLibpq()
	if err != nil {
		return err
	}
	db, err = pgx.NewConnPool(pgx.ConnPoolConfig{ConnConfig: cfg})
	return err
}

func recordMatch(rec matchRecord) {
	if rec.Tier == "" || len(rec.Tier) > 1 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var winner, loser string
	var winpot, losepot int64
	if rec.TwoWins {
		winner, loser = rec.Name2, rec.Name1
		winpot, losepot = rec.Pot2, rec.Pot1
	} else {
		winner, loser = rec.Name1, rec.Name2
		winpot, losepot = rec.Pot1, rec.Pot2
	}
	dur := int(rec.Stop.Sub(rec.Start).Round(time.Second).Seconds())
	_, err := db.ExecEx(ctx, "INSERT INTO matches (winner, loser, winpot, losepot, duration, tier, mode) VALUES ($1, $2, $3, $4, $5, $6, $7)", nil,
		winner, loser, winpot, losepot, dur, rec.Tier, rec.Mode)
	if err != nil {
		log.Printf("error: recording match: %s", err)
	}
}

func setCurrentMatch(rec matchRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	txn, err := db.BeginEx(ctx, nil)
	if err != nil {
		return err
	}
	defer txn.Rollback()
	_, err = txn.ExecEx(ctx, "DELETE FROM current_match", nil)
	if err != nil {
		return err
	}
	_, err = txn.ExecEx(ctx, "INSERT INTO current_match (p1, p2, tier, mode) VALUES ($1, $2, $3, $4)", nil, rec.Name1, rec.Name2, rec.Tier, rec.Mode)
	if err != nil {
		return err
	}
	txn.ExecEx(ctx, "NOTIFY current_match", nil)
	return txn.CommitEx(ctx)
}

func clearCurrentMatch() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecEx(ctx, "DELETE FROM current_match", nil)
	return err
}

func getToken() (*oauth2.Token, error) {
	row := db.QueryRow("SELECT token FROM tokens WHERE name = 'twitch'")
	var blob []byte
	if err := row.Scan(&blob); err == pgx.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	t := new(oauth2.Token)
	err := json.Unmarshal(blob, t)
	return t, err
}

func putToken(t *oauth2.Token) error {
	blob, err := json.Marshal(t)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT INTO tokens (name, token) VALUES ('twitch', $1) ON CONFLICT (name) DO UPDATE SET token = $1", blob)
	return err
}
