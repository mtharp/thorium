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
		// predict
		d := tiers[tierIdx[rec.Tier]]
		wg := wagerFromVector(nn.Predict(d.BetVector(rec)))
		// wager
		wager := bank * wg.Size()
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

func simulateWhale(nn *deep.Neural, recs []*matchRecord, debug bool) float64 {
	bank := whaleStart
	balances := make(sort.Float64Slice, len(recs))
	for i, rec := range recs {
		// predict
		d := tiers[tierIdx[rec.Tier]]
		v := d.BetVector(rec)
		if len(v) != betVectorSize {
			panic("betVectorSize is wrong")
		}
		wg := wagerFromVector(nn.Predict(v))
		// wager
		wager := bank * wg.Size()
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
			log.Printf("%p bank=%f %s wager=%f wp=%d lp=%d chg=%+f [%s]", nn, bank, res, wager, rec.Pot[rec.Winner]/1000, rec.Pot[1-rec.Winner]/1000, change, fmtVec(v))
		}
		bank += change
		/*if bank < simBailout {
			// wow, you lose!
			return -whaleStart + float64(i)
		}*/
		balances[i] = bank
	}
	sort.Sort(balances)
	return balances[len(balances)/4]
}

func simulateWhaleC(nns []*deep.Neural, recs []*matchRecord) {
	bank := whaleStart
	for _, rec := range recs {
		// predict
		d := tiers[tierIdx[rec.Tier]]
		v := d.BetVector(rec)
		if len(v) != betVectorSize {
			panic("betVectorSize is wrong")
		}
		wl := make(wagerList, len(nns))
		for i, nn := range nns {
			wl[i] = wagerFromVector(nn.Predict(v))
		}
		wg := wl.Consensus()
		// wager
		wager := bank * wg.Size() * mmScale
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
		log.Printf("bank=%f %s wager=%f wp=%d lp=%d chg=%+f [%s]", bank, res, wager, rec.Pot[rec.Winner]/1000, rec.Pot[1-rec.Winner]/1000, change, fmtVec(v))
		bank += change
	}
	log.Printf("final: %s", fmtNum(bank))
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
