package main

import (
	"math"
	"sync"
	"time"

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

func newLiveRecord(tier, a, b string) *matchRecord {
	if a > b {
		a, b = b, a
	}
	return &matchRecord{Tier: tier, Name: [2]string{a, b}}
}

func (r *matchRecord) Payoff(wager float64) float64 {
	return wager * float64(r.Pot[1-r.Winner]) / float64(r.Pot[r.Winner])
}

func (r *matchRecord) Response() []float64 {
	switch predResponseSize {
	case 1:
		if r.Winner == 1 {
			return []float64{1.0}
		} else {
			return []float64{-1.0}
		}
	case 2:
		response := []float64{0.0, 0.0}
		response[r.Winner] = 1.0
		return response
	default:
		panic("nah")
	}
}

func getRecords(table string, since time.Time) (tierRecs map[string][]*matchRecord, ts time.Time, err error) {
	tierRecs = make(map[string][]*matchRecord)
	cfg, err := pgx.ParseConnectionString(viper.GetString("db.url"))
	if err != nil {
		return
	}
	conn, err := pgx.Connect(cfg)
	if err != nil {
		return
	}
	defer conn.Close()
	rows, err := conn.Query("SELECT ts, tier, winner, loser, winpot, losepot, duration FROM "+table+" WHERE mode = 'matchmaking' AND ts > $1 ORDER BY ts", since)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var tier, winner, loser string
		var winpot, losepot int64
		var duration int
		var recTS time.Time
		if err = rows.Scan(&recTS, &tier, &winner, &loser, &winpot, &losepot, &duration); err != nil {
			return
		}
		if winpot == 0 || losepot == 0 || duration == 0 {
			continue
		}
		if _, ok := tierIdx[tier]; !ok {
			continue
		}
		tierRecs[tier] = append(tierRecs[tier], newRecord(tier, winner, loser, winpot, losepot, duration))
		if recTS.After(ts) {
			ts = recTS
		}
	}
	err = rows.Err()
	return
}

// stats

type charStats struct {
	Wins, Losses      float64
	WinTime, LoseTime float64
	Elo               float64
	Favor             float64
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

func (s *charStats) CrowdFavor() float64 {
	return s.Favor / (s.Wins + s.Losses)
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

		winpot, losepot := float64(rec.Pot[rec.Winner]), float64(rec.Pot[1-rec.Winner])
		swin.Favor = winpot / losepot
		slose.Favor = losepot / winpot

		expected := 1 / (1 + math.Pow(10, (slose.Elo-swin.Elo)/400))
		change := kFactor * (1 - expected)
		swin.Elo += change
		slose.Elo -= change

		m[rec.Name[rec.Winner]] = swin
		m[rec.Name[1-rec.Winner]] = slose
	}
}
