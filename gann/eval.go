package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx"
	deep "github.com/patrikeh/go-deep"
	"github.com/patrikeh/go-deep/training"
	"github.com/spf13/viper"
)

const kFactor = 32

type matchRecord struct {
	Winner, Loser   string
	WinPot, LosePot int64
	Duration        int
}

func (rec matchRecord) Simulated() simRecord {
	a, b := rec.Winner, rec.Loser
	outcome := -1
	if a > b {
		a, b = b, a
		outcome = 1
	}
	return simRecord{a, b, rec.WinPot, rec.LosePot, outcome}
}

type simRecord struct {
	A, B            string
	WinPot, LosePot int64
	Outcome         int
}

func (s simRecord) Payoff(wager float64) float64 {
	winpot := float64(s.WinPot) + wager
	return wager * float64(s.LosePot) / winpot
}

func makeVector(astat, bstat *charStats) []float64 {
	rateDelta := astat.WinRate - bstat.WinRate
	winDelta := astat.AvgWinTime - bstat.AvgWinTime
	loseDelta := bstat.LoseTime - astat.LoseTime
	bWins := 1 / (1 + math.Pow(10, (astat.Elo-bstat.Elo)/400))
	bWins = 2*bWins - 1
	agames := astat.Wins + astat.Losses
	bgames := bstat.Wins + bstat.Losses
	leastGames := agames
	if bgames < leastGames {
		leastGames = bgames
	}
	return []float64{rateDelta, winDelta, loseDelta, bWins, float64(leastGames)}
}

func getRecords(tier, table string) ([]*matchRecord, error) {
	cfg, err := pgx.ParseConnectionString(viper.GetString("db.url"))
	if err != nil {
		return nil, err
	}
	conn, err := pgx.Connect(cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	//rows, err := conn.Query("SELECT winner, loser, winpot, losepot, duration FROM (SELECT * FROM imported_matches UNION ALL SELECT * FROM matches) x WHERE mode = 'matchmaking' AND tier = $1", tier)
	rows, err := conn.Query("SELECT winner, loser, winpot, losepot, duration FROM "+table+" WHERE mode = 'matchmaking' AND tier = $1", tier)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recs []*matchRecord
	for rows.Next() {
		rec := new(matchRecord)
		if err := rows.Scan(&rec.Winner, &rec.Loser, &rec.WinPot, &rec.LosePot, &rec.Duration); err != nil {
			return nil, err
		}
		recs = append(recs, rec)
	}
	return recs, rows.Err()
}

type charStats struct {
	Wins, Losses      float64
	WinTime, LoseTime float64

	AvgWinTime, AvgLoseTime float64
	WinRate                 float64
	Elo                     float64
}

func updateCharStats(charStat map[string]*charStats, recs []*matchRecord) {
	for _, rec := range recs {
		swin := charStat[rec.Winner]
		if swin == nil {
			swin = &charStats{Elo: 3000}
		}
		slose := charStat[rec.Loser]
		if slose == nil {
			slose = &charStats{Elo: 3000}
		}
		swin.Wins++
		swin.WinTime += float64(rec.Duration)
		slose.Losses++
		slose.LoseTime += float64(rec.Duration)

		expected := 1 / (1 + math.Pow(10, (slose.Elo-swin.Elo)/400))
		change := kFactor * (1 - expected)
		swin.Elo += change
		slose.Elo -= change

		charStat[rec.Winner] = swin
		charStat[rec.Loser] = slose
	}
	for _, s := range charStat {
		if s.Wins != 0 {
			s.AvgWinTime = s.WinTime / s.Wins
		}
		if s.Losses != 0 {
			s.AvgLoseTime = s.LoseTime / s.Losses
		}
		s.WinRate = s.Wins / (s.Wins + s.Losses)
	}
}

func main() {
	viper.SetConfigName("thorium")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln("error:", err)
	}
	tier := os.Args[1]
	recs, err := getRecords(tier, "imported_matches")
	if err != nil {
		log.Fatalln("error:", err)
	}
	charStat := make(map[string]*charStats)
	updateCharStats(charStat, recs)
	var e training.Examples
	for _, rec := range recs {
		s := rec.Simulated()
		j, k := 0.0, 0.0
		if s.Outcome > 0 {
			k = 1.0
		} else {
			j = 1.0
		}
		e = append(e, training.Example{
			Input:    makeVector(charStat[s.A], charStat[s.B]),
			Response: []float64{j, k},
		})
	}
	var nn *deep.Neural
	filename := "nn" + tier + ".dat"
	blob, err := ioutil.ReadFile(filename)
	if err == nil {
		nn, err = deep.Unmarshal(blob)
		if err != nil {
			panic(err)
		}
	} else {
		nn = deep.NewNeural(&deep.Config{
			Inputs:     5,
			Layout:     []int{5, 3, 2},
			Activation: deep.ActivationSigmoid,
			Mode:       deep.ModeMultiClass,
			Weight:     deep.NewNormal(1.0, 0.0),
			Bias:       true,
		})
		optimizer := training.NewAdam(0.001, 0.9, 0.999, 1e-8)
		trainer := training.NewBatchTrainer(optimizer, 1, 200, 6)
		x, y := e.Split(0.75)
		trainer.Train(nn, x, y, 250)
		blob, err := nn.Marshal()
		if err != nil {
			panic(err)
		}
		if err := ioutil.WriteFile(filename, blob, 0644); err != nil {
			panic(err)
		}
	}

	recs, err = getRecords(tier, "matches")
	if err != nil {
		log.Fatalln("error:", err)
	}
	updateCharStats(charStat, recs)
	if true {
		watchAndRun(0.5, charStat, nn, tier)
	} else {
		sim := make([]simRecord, len(recs))
		for i, rec := range recs {
			sim[i] = rec.Simulated()
		}
		log.Printf("%d sim records", len(sim))
		stride := 25
		width := 100
		for baseBet := 0.20; baseBet <= 0.80; baseBet += 0.10 {
			for z := 0; z+width <= len(sim); z += stride {
				var results sort.Float64Slice
				for x := 0; x < 100000; x++ {
					selected := sim[z : z+width]
					rand.Shuffle(len(selected), func(i, j int) { selected[i], selected[j] = selected[j], selected[i] })
					result := simulateRun(baseBet, charStat, nn, sim[z:z+width])
					//log.Printf("%3d %.2f %f", z, baseBet, result)
					results = append(results, result)
				}
				sort.Sort(results)
				median := results[len(results)/2]
				if len(results)%2 == 0 {
					median = (median + results[len(results)/2-1]) / 2
				}
				fmt.Printf("%.2f,%f\n", baseBet, median)
			}
		}
	}
}

func simulateRun(baseBet float64, charStat map[string]*charStats, nn *deep.Neural, sim []simRecord) float64 {
	bailout := 100.0
	bank := bailout
	for _, s := range sim {
		// predict
		v := makeVector(charStat[s.A], charStat[s.B])
		o := nn.Predict(v)
		j, k := o[0], o[1]
		wk := j
		if k > j {
			wk = k
		}
		// wager
		wager := bank * baseBet * wk
		if bank-wager < bailout || wager > bank {
			wager = bank
		}
		// outcome
		change := -wager
		//res := "lose"
		if (s.Outcome > 0) == (k > j) {
			// win
			change = s.Payoff(wager)
			//res = "win"
		}
		//log.Printf("bank=%f pred=%f/%f %s wager=%f wp=%d lp=%d chg=%+f bank=%f", bank, j, k, res, wager, s.WinPot/1000, s.LosePot/1000, change, bank+change)
		bank += change
		if bank < bailout {
			bank = bailout
		}
	}
	return bank
}

func watchAndRun(baseBet float64, charStat map[string]*charStats, nn *deep.Neural, tier string) {
	var p1name, p2name string
	var sbs struct {
		P1, P2 string
		Bank   int64
	}
	var mst struct{ P1, P2, Tier string }
	for {
		time.Sleep(1 * time.Second)
		blob, _ := ioutil.ReadFile("/tmp/sbstate.json")
		json.Unmarshal(blob, &sbs)
		if sbs.P1 == p1name && sbs.P2 == p2name {
			continue
		}
		p1name = sbs.P1
		p2name = sbs.P2
		for mst.P1 != p1name || mst.P2 != p2name {
			blob, _ = ioutil.ReadFile("/tmp/mstate.json")
			json.Unmarshal(blob, &mst)
			time.Sleep(time.Second)
		}
		if mst.Tier != tier {
			log.Printf(".")
			continue
		}
		astat := charStat[p1name]
		bstat := charStat[p2name]
		if astat == nil || bstat == nil {
			log.Printf("%q %q no data", p1name, p2name)
			continue
		}
		predicted := nn.Predict(makeVector(astat, bstat))[0]
		psign := 1.0
		if predicted < 0 {
			psign = -1.0
		}
		wager := 0.1 * float64(sbs.Bank) * predicted * psign
		betOn := p2name
		if psign < 0 {
			betOn = p1name
		}
		log.Printf("bet %q %d", betOn, int(wager))
	}
}
