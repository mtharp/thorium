package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"sort"

	deep "github.com/patrikeh/go-deep"
	"github.com/patrikeh/go-deep/training"
	"github.com/spf13/viper"
)

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
	chars := make(charStatsMap)
	chars.Update(recs)
	e := make(training.Examples, len(recs))
	for i, rec := range recs {
		e[i] = training.Example{
			Input:    chars.Vector(rec),
			Response: rec.Response(),
		}
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
	chars.Update(recs)
	stride := 25
	width := 100
	for baseBet := 0.20; baseBet <= 0.80; baseBet += 0.20 {
		for z := 0; z+width <= len(recs); z += stride {
			var results sort.Float64Slice
			for x := 0; x < 10000; x++ {
				selected := recs[z : z+width]
				rand.Shuffle(len(selected), func(i, j int) { selected[i], selected[j] = selected[j], selected[i] })
				result := simulateRun(baseBet, chars, nn, recs[z:z+width])
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
