package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

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
	if len(os.Args) < 2 {
		log.Fatalln("pred, train")
	}
	switch os.Args[1] {
	case "pred":
		prepData(true)
	case "train":
		workDir := "_bnet"
		if err := os.MkdirAll(workDir, 0755); err != nil {
			log.Fatalln("error:", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			tsig := make(chan os.Signal)
			signal.Notify(tsig, syscall.SIGINT)
			<-tsig
			signal.Stop(tsig)
			cancel()
		}()
		rng := rand.New(rand.NewSource(42))
		allRecs, _, _ := prepData(true)
		recSets := sliceRecs(allRecs)
		for ctx.Err() == nil {
			evalFunc := func(nn *deep.Neural) float64 {
				scores := make(sort.Float64Slice, len(recSets))
				for i, recSet := range recSets {
					scores[i] = simulateWhale(nn, recSet, false, false)
				}
				sort.Sort(scores)
				return scores[len(scores)/2]
			}
			nn, score := train(ctx, betCfg, evalFunc, rng)
			blob, _ := nn.Marshal()
			ioutil.WriteFile(filepath.Join(workDir, fmt.Sprintf("%d.%d.dat", int64(score), time.Now().Unix())), blob, 0644)
		}
	case "bet":
		nn, err := netFromFiles("_bnet")
		if err != nil {
			log.Fatalln("error:", err)
		}
		watchAndRun(nn)
	}
}

func sliceRecs(recs []*matchRecord) [][]*matchRecord {
	sliceCount := 8
	recSets := make([][]*matchRecord, sliceCount)
	for i, rec := range recs {
		j := i % sliceCount
		recSets[j] = append(recSets[j], rec)
	}
	return recSets
}

func prepData(split bool) (allRecs []*matchRecord, ts time.Time, avgPot float64) {
	rng := rand.New(rand.NewSource(123))
	tierRecs, ts, avgPot, err := getRecords("all_matches", time.Time{}, 0, false)
	if err != nil {
		log.Fatalln("error:", err)
	}
	for tier, i := range tierIdx {
		recs := tierRecs[tier]
		chars := make(charStatsMap)
		var toTrain, forStats []*matchRecord
		if split {
			for _, rec := range recs {
				if rng.Int63()%2 == 0 {
					toTrain = append(toTrain, rec)
				} else {
					forStats = append(forStats, rec)
				}
			}
		} else {
			forStats = recs
		}
		chars.Update(forStats)
		allRecs = append(allRecs, toTrain...)
		tiers[i] = &tierData{recs: toTrain, chars: chars}
	}
	return
}
