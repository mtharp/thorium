package main

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx"
	"github.com/spf13/viper"
)

func connectDB() (*pgx.ConnPool, error) {
	cfg, err := pgx.ParseConnectionString(viper.GetString("db.url"))
	if err != nil {
		return nil, err
	}
	return pgx.NewConnPool(pgx.ConnPoolConfig{ConnConfig: cfg})
}

func recordMatch(db *pgx.ConnPool, rec matchRecord) {
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
