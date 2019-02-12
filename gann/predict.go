package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"runtime"

	deep "github.com/patrikeh/go-deep"
	"github.com/patrikeh/go-deep/training"
)

var predGens = map[string]int{
	"P": 10000,
	"B": 2500,
	"A": 1500,
	"S": 2500,
	"X": 3500,
}

func (d *tierData) makePredictor(tier string) error {
	predCfg.Inputs = len(d.chars.PredVector(d.recs[0].Names()))
	filename := "_pnet/" + tier + ".dat"
	if err := os.MkdirAll("_pnet", 0755); err != nil {
		return err
	}
	blob, err := ioutil.ReadFile(filename)
	if err == nil {
		dump := new(deep.Dump)
		if err := json.Unmarshal(blob, dump); err != nil {
			return err
		}
		d.setPred(dump)
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	e := make(training.Examples, len(d.recs))
	for i, rec := range d.recs {
		e[i] = training.Example{
			Input:    d.chars.PredVector(rec.Names()),
			Response: rec.Response(),
		}
	}
	if len(e[0].Response) != predResponseSize {
		panic("fix predResponseSize")
	}
	// copy config so that the defaults aren't overwritten
	ncfg := new(deep.Config)
	*ncfg = *predCfg
	nn := deep.NewNeural(ncfg)
	optimizer := training.NewAdam(0.001, 0.9, 0.999, 1e-8)
	trainer := training.NewBatchTrainer(optimizer, 1, 200, runtime.GOMAXPROCS(0))
	x, y := e.Split(0.5)
	gens := predGens[d.recs[0].Tier]
	trainer.Train(nn, x, y, gens)
	dump := nn.Dump()
	blob, err = json.Marshal(dump)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(filename, blob, 0644); err != nil {
		return err
	}
	d.setPred(dump)
	return nil
}

func leastGames(astat, bstat *charStats) float64 {
	agames := astat.Wins + astat.Losses
	bgames := bstat.Wins + bstat.Losses
	if bgames < agames {
		return bgames
	}
	return agames
}

func (m charStatsMap) PredVector(a, b string) []float64 {
	astat, bstat := m[a], m[b]
	rateDelta := astat.WinRate() - bstat.WinRate()
	winDelta := astat.AvgWinTime() - bstat.AvgWinTime()
	loseDelta := bstat.AvgLoseTime() - astat.AvgLoseTime()
	games := leastGames(astat, bstat)
	return []float64{rateDelta, winDelta, loseDelta, games, m.Graph3(a, b)}
}

func (d *tierData) setPred(dump *deep.Dump) {
	// Neural objects are not goroutine-safe so use a pool instead
	d.ppool.New = func() interface{} {
		return deep.FromDump(dump)
	}
}

func (d *tierData) Predict(a, b string) []float64 {
	nn := d.ppool.Get().(*deep.Neural)
	prediction := nn.Predict(d.chars.PredVector(a, b))
	d.ppool.Put(nn)
	return prediction
}
