package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"

	deep "github.com/patrikeh/go-deep"
)

const (
	simBailout = 100.0
	baseBet    = 1.0
)

func shuffleRecords(recs []*matchRecord) {
	rand.Shuffle(len(recs), func(i, j int) { recs[i], recs[j] = recs[j], recs[i] })
}

func simulateBailout(nn *deep.Neural, recs []*matchRecord) (score float64) {
	bank := simBailout
	for _, rec := range recs {
		// predict
		d := tiers[tierIdx[rec.Tier]]
		wg := wagerFromVector(nn.Predict(d.BetVector(rec)))
		// wager
		wager := bank * baseBet * wg.Size()
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

const whaleStart = 5e6

func simulateWhale(nn *deep.Neural, recs []*matchRecord, debug bool) (score float64) {
	bank := whaleStart
	for _, rec := range recs {
		// predict
		d := tiers[tierIdx[rec.Tier]]
		v := d.BetVector(rec)
		if len(v) != betVectorSize {
			panic("betVectorSize is wrong")
		}
		wg := wagerFromVector(nn.Predict(v))
		// wager
		wager := bank * baseBet * wg.Size()
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
		score += change
		if debug {
			//log.Printf("%p score=%f wager=%f vec %s", nn, score, wager, fmtVec(v))
			log.Printf("%p bank=%f %s wager=%f wp=%d lp=%d chg=%+f score=%f [%s]", nn, bank, res, wager, rec.Pot[rec.Winner]/1000, rec.Pot[1-rec.Winner]/1000, change, score, fmtVec(v))
		}
		bank += change
		if bank < simBailout {
			// wow, you lose!
			break
		}
		score++ // reward for not dying
	}
	return
}

func fmtVec(x []float64) string {
	var w []string
	for _, y := range x {
		w = append(w, fmt.Sprintf("%0.3f", y))
	}
	return strings.Join(w, " ")
}

const betVectorSize = predResponseSize + 3

func (d *tierData) BetVector(rec *matchRecord) []float64 {
	rec.bvo.Do(func() {
		a, b := rec.Name[0], rec.Name[1]
		astat := d.chars[a]
		bstat := d.chars[b]
		rateDelta := astat.WinRate() - bstat.WinRate()
		pred := d.Predict(a, b)
		tier := float64(tierIdx[rec.Tier])
		favor := astat.CrowdFavor() - bstat.CrowdFavor()
		rec.bvc = append([]float64{rateDelta, tier, favor}, pred...)
	})
	// NB don't append() to cached value or concurrent callers will stomp each other's "unused" capacity
	return rec.bvc
}
