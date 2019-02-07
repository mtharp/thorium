package main

import (
	"fmt"
	"io"
	"math/rand"
	"strings"

	deep "github.com/patrikeh/go-deep"
)

const (
	bailout = 100.0
	baseBet = 1.0
)

var debug = false

func shuffleRecords(recs []*matchRecord) {
	rand.Shuffle(len(recs), func(i, j int) { recs[i], recs[j] = recs[j], recs[i] })
}

func simulateRun(nn *deep.Neural, recs []*matchRecord, debug io.Writer) (score float64) {
	bank := bailout
	for _, rec := range recs {
		// predict
		d := tiers[tierIdx[rec.Tier]]
		v := d.BetVector(rec)
		o := nn.Predict(v)
		j, k := o[0], o[1]
		wk := j
		if k > j {
			wk = k
		}
		if wk < 0 {
			wk = 0
		}
		// wager
		wager := bank * baseBet * wk
		if bank-wager < bailout || wager > bank {
			wager = bank
		}
		// outcome
		change := -wager
		//res := "lose"
		if (rec.Winner == 1) == (k > j) {
			// win
			change = rec.Payoff(wager)
			//res = "win"
		}
		score += change
		if debug != nil {
			fmt.Fprintf(debug, "%p %f %+f %s %f/%f\n", nn, score, change, fmtVec(v), j, k)
			//log.Printf("%p bank=%f pred=%f/%f %s wager=%f wp=%d lp=%d chg=%+f score=%f", nn, bank, j, k, res, wager, rec.Pot[rec.Winner]/1000, rec.Pot[1-rec.Winner]/1000, change, score)
		}
		bank += change
		if bank < bailout {
			bank = bailout
		}
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

func (d *tierData) BetVector(rec *matchRecord) []float64 {
	rec.bvo.Do(func() {
		a, b := rec.Name[0], rec.Name[1]
		astat := d.chars[a]
		bstat := d.chars[b]
		rateDelta := astat.WinRate() - bstat.WinRate()
		eloDelta := bstat.Elo - astat.Elo
		pred := d.Predict(a, b)
		tier := float64(tierIdx[rec.Tier])
		rec.bvc = []float64{rateDelta, eloDelta, pred[0], pred[1], tier}
	})
	return rec.bvc
}

const betVectorSize = 5
