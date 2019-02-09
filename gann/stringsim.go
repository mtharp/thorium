package main

import (
	"fmt"
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
		betSize, predictB := wagerFromVector(nn.Predict(d.BetVector(rec, bank)))
		// wager
		wager := bank * baseBet * betSize
		if bank-wager < simBailout || wager > bank {
			wager = bank
		}
		// outcome
		change := -wager
		//res := "lose"
		if (rec.Winner == 1) == predictB {
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

func simulateWhale(nn *deep.Neural, recs []*matchRecord) (score float64) {
	bank := whaleStart
	for _, rec := range recs {
		// predict
		d := tiers[tierIdx[rec.Tier]]
		betSize, predictB := wagerFromVector(nn.Predict(d.BetVector(rec, bank)))
		// wager
		wager := bank * baseBet * betSize
		if bank-wager < simBailout || wager > bank {
			wager = bank
		}
		// outcome
		change := -wager
		//res := "lose"
		if (rec.Winner == 1) == predictB {
			// win
			change = rec.Payoff(wager)
			//res = "win"
		}
		score += change
		//log.Printf("%p bank=%f pred=%f/%f %s wager=%f wp=%d lp=%d chg=%+f score=%f", nn, bank, j, k, res, wager, rec.Pot[rec.Winner]/1000, rec.Pot[1-rec.Winner]/1000, change, score)
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

func wagerFromVector(o []float64) (betSize float64, predictB bool) {
	switch len(o) {
	case 2:
		j, k := o[0], o[1]
		betSize = j
		if k > j {
			betSize = k
			predictB = true
		}
	case 1:
		betSize = o[0]
		if betSize > 0 {
			predictB = true
		} else {
			betSize = -betSize
		}
	default:
		panic("invalid output size")
	}
	if betSize < 0.002 {
		betSize = 0
	}
	return
}

func (d *tierData) BetVector(rec *matchRecord, bank float64) []float64 {
	rec.bvo.Do(func() {
		a, b := rec.Name[0], rec.Name[1]
		astat := d.chars[a]
		bstat := d.chars[b]
		rateDelta := astat.WinRate() - bstat.WinRate()
		eloDelta := bstat.Elo - astat.Elo
		pred := d.Predict(a, b)
		tier := float64(tierIdx[rec.Tier])
		rec.bvc = append([]float64{rateDelta, eloDelta, tier}, pred...)
	})
	if len(rec.bvc) == betVectorSize {
		return rec.bvc
	} else if len(rec.bvc)+1 == betVectorSize {
		v := make([]float64, len(rec.bvc), betVectorSize)
		copy(v, rec.bvc)
		return append(v, bank)
	} else {
		panic("fix betVectorSize")
	}
}
