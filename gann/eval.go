package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
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
	var training, meta bool
	var drange int
	table := "all_matches"
	if len(os.Args) > 1 {
		training = true
		switch os.Args[1] {
		case "predictor":
			drange = 1
			table = "imported_matches"
		case "train":
			drange = 2
		case "meta":
			meta = true
		default:
			log.Fatalln("predictor, train, or meta?")
		}
	}
	tierRecs, ts, err := getRecords(table, time.Time{}, drange)
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
		if err := d.makePredictor(tier); err != nil {
			log.Fatalln("error:", err)
		}
		tiers[i] = d
	}
	if training {
		if drange == 1 {
			return
		}
		var startPop []*deep.Neural
		var seed int64 = 42
		workDir := "_bnet"
		if meta {
			seed = time.Now().UnixNano()
			startPop, err = netsFromFiles(workDir, population)
			if err != nil {
				log.Fatalln("error:", err)
			}
			workDir = "_meta"
		}
		if err := os.MkdirAll(workDir, 0755); err != nil {
			log.Fatalln("error:", err)
		}
		for {
			rng := rand.New(rand.NewSource(seed))
			recSets := sliceRecs(rng, allRecs)
			evalFunc := func(nn *deep.Neural, debug bool) float64 {
				if debug {
					log.Println("WAHHH")
				}
				scores := make(sort.Float64Slice, len(recSets))
				for i, recSet := range recSets {
					scores[i] = simulateWhale(nn, recSet, debug)
				}
				// median
				sort.Sort(scores)
				s := scores[len(scores)/2]
				if len(scores)%2 == 0 {
					s = (s + scores[len(scores)/2-1]) / 2
				}
				return s
			}
			var shuf shufFunc
			if meta {
				shuf = func() { recSets = sliceRecs(rng, allRecs) }
			}
			nn, score := train(betCfg, evalFunc, shuf, rng, startPop)
			blob, _ := nn.Marshal()
			ioutil.WriteFile(filepath.Join(workDir, fmt.Sprintf("%d.%d.dat", int64(score), time.Now().Unix())), blob, 0644)
		}
	} else {
		nns, err := netsFromFiles("_meta", consensusNets)
		if err != nil {
			log.Fatalln("error:", err)
		}
		watchAndRun(nns, ts)
	}
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
