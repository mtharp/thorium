package main

import (
	"math"
	"sync"

	"github.com/jackc/pgx"
	"github.com/spf13/viper"
)

const (
	defaultElo = 2000
	kFactor    = 32
)

// records

type matchRecord struct {
	Tier     string
	Name     [2]string
	Pot      [2]int64
	Winner   int
	Duration int

	bvo sync.Once
	bvc []float64
}

func newRecord(tier, winner, loser string, winpot, losepot int64, duration int) *matchRecord {
	r := &matchRecord{
		Tier:     tier,
		Name:     [2]string{winner, loser},
		Pot:      [2]int64{winpot, losepot},
		Winner:   0,
		Duration: duration,
	}
	if winner < loser {
		r.Name[0], r.Name[1] = r.Name[1], r.Name[0]
		r.Pot[0], r.Pot[1] = r.Pot[1], r.Pot[0]
		r.Winner = 1
	}
	return r
}

func (r *matchRecord) Payoff(wager float64) float64 {
	return wager * float64(r.Pot[1-r.Winner]) / float64(r.Pot[r.Winner])
}

func (r *matchRecord) Response() []float64 {
	response := []float64{0.0, 0.0}
	response[r.Winner] = 1.0
	return response
}

func getRecords(table string) (map[string][]*matchRecord, error) {
	tierRecs := make(map[string][]*matchRecord)
	cfg, err := pgx.ParseConnectionString(viper.GetString("db.url"))
	if err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	rows, err := conn.Query("SELECT tier, winner, loser, winpot, losepot, duration FROM " + table + " WHERE mode = 'matchmaking' ORDER BY ts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tier, winner, loser string
		var winpot, losepot int64
		var duration int
		if err := rows.Scan(&tier, &winner, &loser, &winpot, &losepot, &duration); err != nil {
			return nil, err
		}
		if winpot == 0 || losepot == 0 || duration == 0 {
			continue
		}
		if _, ok := tierIdx[tier]; !ok {
			continue
		}
		tierRecs[tier] = append(tierRecs[tier], newRecord(tier, winner, loser, winpot, losepot, duration))
	}
	return tierRecs, rows.Err()
}

// stats

type charStats struct {
	Wins, Losses      float64
	WinTime, LoseTime float64
	Elo               float64
}

func (s *charStats) AvgWinTime() float64 {
	if s.Wins == 0 {
		return 600
	}
	return s.WinTime / s.Wins
}

func (s *charStats) AvgLoseTime() float64 {
	if s.Losses == 0 {
		return 600
	}
	return s.LoseTime / s.Losses
}

func (s *charStats) WinRate() float64 {
	return s.Wins / (s.Wins + s.Losses)
}

type charStatsMap map[string]*charStats

func (m charStatsMap) Update(recs []*matchRecord) {
	for _, rec := range recs {
		swin := m[rec.Name[rec.Winner]]
		if swin == nil {
			swin = &charStats{Elo: defaultElo}
		}
		slose := m[rec.Name[1-rec.Winner]]
		if slose == nil {
			slose = &charStats{Elo: defaultElo}
		}
		swin.Wins++
		swin.WinTime += float64(rec.Duration)
		slose.Losses++
		slose.LoseTime += float64(rec.Duration)

		expected := 1 / (1 + math.Pow(10, (slose.Elo-swin.Elo)/400))
		change := kFactor * (1 - expected)
		swin.Elo += change
		slose.Elo -= change

		m[rec.Name[rec.Winner]] = swin
		m[rec.Name[1-rec.Winner]] = slose
	}
}
