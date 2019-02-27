package main

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strings"

	deep "github.com/patrikeh/go-deep"
)

const (
	simBailout = 100.0
)

func shuffleRecords(recs []*matchRecord) {
	rand.Shuffle(len(recs), func(i, j int) { recs[i], recs[j] = recs[j], recs[i] })
}

func simulateBailout(nn *deep.Neural, recs []*matchRecord) (score float64) {
	bank := simBailout
	for _, rec := range recs {
		d := tiers[tierIdx[rec.Tier]]
		wg := wagerFromVector(nn.Predict(d.BetVector(rec, bank)))
		// wager
		wager := bank * wg.Size() * mmScale
		if bank-wager < simBailout || wager > bank {
			wager = bank
		}
		// outcome
		change := -wager
		//res := "lose"
		if (rec.Winner == 1) == wg.PredictB() {
			// win
			change = rec.Payoff(wager)
			//res = "win"
		}
		//fmt.Fprintf(debug, "bank=%f score=%f wager=%f change=%f vec=[%s]\n", bank, score, wager, change, fmtVec(v))
		//log.Printf("%p bank=%f %s wager=%f wp=%d lp=%d chg=%+f score=%f", nn, bank, res, wager, rec.Pot[rec.Winner]/1000, rec.Pot[1-rec.Winner]/1000, change, score)
		score += change
		bank += change
		if bank < simBailout {
			bank = simBailout
		}
	}
	return
}

const whaleStart = 1e6

func simulateWhale(nn *deep.Neural, recs []*matchRecord, scale, debug bool) float64 {
	bank := whaleStart
	balances := make(sort.Float64Slice, len(recs))
	for i, rec := range recs {
		// predict
		d := tiers[tierIdx[rec.Tier]]
		v := d.BetVector(rec, bank)
		if v == nil {
			// no data
			balances[i] = bank
			continue
		} else if len(v) != betVectorSize {
			panic("betVectorSize is wrong")
		}
		wg := wagerFromVector(nn.Predict(v))
		// wager
		wager := bank * wg.Size()
		if scale {
			wager *= mmScale
		}
		if bank-wager < simBailout || wager > bank {
			wager = bank
		}
		// outcome
		change := -wager
		res := "lose"
		if (rec.Winner == 1) == wg.PredictB() {
			// win
			change = rec.Payoff(wager)
			res = "win "
		}
		if debug {
			//log.Printf("%p score=%f wager=%f vec %s", nn, score, wager, fmtVec(v))
			log.Printf("%p bank=%f %s wager=%f po=%.3f ppo=%.3f chg=%+f [%s]", nn, bank, res, wager, rec.Payoff(1), change, fmtVec(v))
		}
		bank += change
		if bank < simBailout {
			// wow, you lose!
			return -whaleStart + float64(i)
		}
		balances[i] = bank
	}
	sort.Sort(balances)
	return balances[len(balances)/10]
	return bank
}

func fmtVec(x []float64) string {
	var w []string
	for _, y := range x {
		w = append(w, fmt.Sprintf("%0.3f", y))
	}
	return strings.Join(w, " ")
}

const betVectorSize = 6

func (d *tierData) BetVector(rec *matchRecord, bank float64) []float64 {
	rec.bvo.Do(func() {
		a, b := rec.Name[0], rec.Name[1]
		astat, bstat := d.chars[a], d.chars[b]
		if astat == nil || bstat == nil {
			return
		}
		rec.bvc = []float64{
			astat.WinRate() - bstat.WinRate(),
			astat.CrowdFavor() - bstat.CrowdFavor(),
			astat.AvgWinTime() - bstat.AvgWinTime(),
			leastGames(astat, bstat),
			bank / rec.PotAvg,
			float64(tierIdx[rec.Tier]),
		}
	})
	return rec.bvc
}

func leastGames(astat, bstat *charStats) float64 {
	agames := astat.Wins + astat.Losses
	bgames := bstat.Wins + bstat.Losses
	if bgames < agames {
		return bgames
	}
	return agames
}
