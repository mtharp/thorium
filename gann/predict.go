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
	"B": 1500,
	"A": 1500,
	"S": 1500,
	"X": 10000,
}

func (d *tierData) makePredictor(filename string) error {
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
			Input:    d.chars.PredVector(rec),
			Response: rec.Response(),
		}
	}
	nn := deep.NewNeural(&deep.Config{
		Inputs:     len(e[0].Input),
		Layout:     []int{5, 3, len(e[0].Response)},
		Activation: deep.ActivationSigmoid,
		Mode:       deep.ModeMultiClass,
		Weight:     deep.NewNormal(1.0, 0.0),
		Bias:       true,
	})
	optimizer := training.NewAdam(0.001, 0.9, 0.999, 1e-8)
	trainer := training.NewBatchTrainer(optimizer, 1, 200, runtime.GOMAXPROCS(0))
	x, y := e.Split(0.75)
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

func predVector(astat, bstat *charStats) []float64 {
	rateDelta := astat.WinRate() - bstat.WinRate()
	winDelta := astat.AvgWinTime() - bstat.AvgWinTime()
	loseDelta := bstat.AvgLoseTime() - astat.AvgLoseTime()
	eloDelta := bstat.Elo - astat.Elo
	games := leastGames(astat, bstat)
	return []float64{rateDelta, winDelta, loseDelta, eloDelta, games}
}

func (m charStatsMap) PredVector(rec *matchRecord) []float64 {
	return predVector(m[rec.Name[0]], m[rec.Name[1]])
}

func (d *tierData) setPred(dump *deep.Dump) {
	// Neural objects are not goroutine-safe so use a pool instead
	d.ppool.New = func() interface{} {
		return deep.FromDump(dump)
	}
}

func (d *tierData) Predict(a, b string) []float64 {
	astat := d.chars[a]
	bstat := d.chars[b]
	nn := d.ppool.Get().(*deep.Neural)
	prediction := nn.Predict(predVector(astat, bstat))
	d.ppool.Put(nn)
	return prediction
}
