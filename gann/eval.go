package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	var training bool
	table := "all_matches"
	if len(os.Args) > 1 {
		training = true
		table = "imported_matches"
	}
	tierRecs, ts, err := getRecords(table, time.Time{})
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
	var bnn *deep.Neural
	if training {
		var startPop []*deep.Neural
		var seed int64 = 42
		outDir := "_bnet"
		if os.Args[1] == "meta" {
			seed *= 69
			f, err := os.Open(outDir)
			if err != nil {
				log.Fatalln("error:", err)
			}
			names, err := f.Readdirnames(-1)
			if err != nil {
				log.Fatalln("error:", err)
			}
			type fileScore struct {
				name  string
				score float64
			}
			var scores []fileScore
			for _, name := range names {
				score, _ := strconv.ParseInt(strings.Split(name, ".")[0], 10, 64)
				scores = append(scores, fileScore{name, float64(score)})
			}
			sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
			if len(scores) > population {
				scores = scores[:population]
			}
			for _, score := range scores {
				blob, err := ioutil.ReadFile(filepath.Join("_bnet", score.name))
				if err != nil {
					log.Fatalln("error:", err)
				}
				nn, err := deep.Unmarshal(blob)
				if err != nil {
					log.Fatalln("error:", err)
				}
				startPop = append(startPop, nn)
			}
			log.Printf("seeded with %d nets with scores %s - %s", len(startPop), fmtNum(scores[0].score), fmtNum(scores[len(scores)-1].score))
			outDir = "_meta"
		}
		if err := os.MkdirAll(outDir, 0755); err != nil {
			log.Fatalln("error:", err)
		}
		rng := rand.New(rand.NewSource(seed))
		recSets := sliceRecs(rng, allRecs)
		evalFunc := func(nn *deep.Neural, debug io.Writer) float64 {
			scores := make(sort.Float64Slice, len(recSets))
			for i, recSet := range recSets {
				scores[i] = simulateWhale(nn, recSet)
			}
			// median
			sort.Sort(scores)
			return (scores[len(scores)/2] + scores[len(scores)/2-1]) / 2
		}
		for {
			nn, score := train(betCfg, evalFunc, rng, startPop)
			if score > 100e9 {
				blob, _ := nn.Marshal()
				ioutil.WriteFile(filepath.Join(outDir, fmt.Sprintf("%d.%d.dat", int64(score), time.Now().Unix())), blob, 0644)
			}
		}
	} else {
		f, err := os.Open("_meta")
		if err != nil {
			log.Fatalln("error:", err)
		}
		names, err := f.Readdirnames(-1)
		if err != nil {
			log.Fatalln("error:", err)
		}
		var bestScore int64
		var bestName string
		for _, name := range names {
			score, _ := strconv.ParseInt(strings.Split(name, ".")[0], 10, 64)
			if score > bestScore {
				bestScore = score
				bestName = name
			}
		}
		if bestName == "" {
			log.Fatalln("no files")
		}
		blob, err := ioutil.ReadFile(filepath.Join("_meta", bestName))
		if err != nil {
			log.Fatalln("error:", err)
		}
		bnn, err = deep.Unmarshal(blob)
		if err != nil {
			log.Fatalln("error:", err)
		}
		log.Printf("using _meta/%s score=%s", bestName, fmtNum(float64(bestScore)))
		watchAndRun(bnn, ts)
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
