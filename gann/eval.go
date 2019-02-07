package main

import (
	"io"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync"

	deep "github.com/patrikeh/go-deep"
	"github.com/spf13/viper"
)

type tierData struct {
	recs  []*matchRecord
	chars charStatsMap

	weights [][][]float64
	ppool   sync.Pool
}

var (
	tierIdx = map[string]int{
		"P": 0,
		"B": 1,
		"A": 2,
		"S": 3,
		"X": 4,
	}
	tiers [5]*tierData
)

func main() {
	viper.SetConfigName("thorium")
	viper.AddConfigPath(".")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatalln("error:", err)
	}
	tierRecs, err := getRecords("imported_matches")
	if err != nil {
		log.Fatalln("error:", err)
	}
	var allRecs []*matchRecord
	for tier, i := range tierIdx {
		recs := tierRecs[tier]
		allRecs = append(allRecs, recs...)
		chars := make(charStatsMap)
		chars.Update(recs)
		d := &tierData{recs: recs, chars: chars}
		if err := d.makePredictor("pred" + tier + ".dat"); err != nil {
			log.Fatalln("error:", err)
		}
		tiers[i] = d
	}
	ncfg := &deep.Config{
		Inputs:     betVectorSize,
		Layout:     []int{6, 4, 2},
		Activation: deep.ActivationSigmoid,
		Mode:       deep.ModeRegression,
		Weight:     deep.NewNormal(100.0, 0.0),
	}
	rng := rand.New(rand.NewSource(42))
	recSets := sliceRecs(rng, allRecs)
	evalFunc := func(nn *deep.Neural, debug io.Writer) float64 {
		scores := make(sort.Float64Slice, len(recSets))
		for i, recSet := range recSets {
			scores[i] = simulateRun(nn, recSet, debug)
		}
		// median
		sort.Sort(scores)
		return (scores[len(scores)/2] + scores[len(scores)/2-1]) / 2
	}
	train(ncfg, evalFunc, rng)
}

func sliceRecs(rng *rand.Rand, recs []*matchRecord) [][]*matchRecord {
	k := int(math.Sqrt(float64(len(recs))) - 1)
	sliceCount := k
	sliceSize := k
	recSets := make([][]*matchRecord, sliceCount)
	for i := range recSets {
		rng.Shuffle(len(recs), func(j, k int) {
			recs[j], recs[k] = recs[k], recs[j]
		})
		recSet := make([]*matchRecord, sliceSize)
		copy(recSet, recs)
		recSets[i] = recSet
	}
	return recSets
}
