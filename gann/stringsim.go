package main

import (
	"log"

	deep "github.com/patrikeh/go-deep"
)

const debug = false

func simulateRun(baseBet float64, chars charStatsMap, nn *deep.Neural, recs []*matchRecord) float64 {
	bailout := 100.0
	bank := bailout
	for _, rec := range recs {
		// predict
		v := makeVector(chars[rec.Name[0]], chars[rec.Name[1]])
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
		res := "lose"
		if (rec.Winner == 1) == (k > j) {
			// win
			change = rec.Payoff(wager)
			res = "win"
		}
		if debug {
			log.Printf("bank=%f pred=%f/%f %s wager=%f wp=%d lp=%d chg=%+f bank=%f",
				bank, j, k, res, wager, rec.Pot[rec.Winner]/1000, rec.Pot[1-rec.Winner]/1000, change, bank+change)
		}
		bank += change
		if bank < bailout {
			bank = bailout
		}
	}
	return bank
}
