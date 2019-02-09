package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	deep "github.com/patrikeh/go-deep"
)

func netsFromFiles(dirname string, count int) ([]*deep.Neural, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	var scores []fileScore
	for _, name := range names {
		score, _ := strconv.ParseInt(strings.Split(name, ".")[0], 10, 64)
		scores = append(scores, fileScore{name, float64(score)})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	if len(scores) > count {
		scores = scores[:count]
	}
	var ret []*deep.Neural
	for _, score := range scores {
		blob, err := ioutil.ReadFile(filepath.Join(dirname, score.name))
		if err != nil {
			return nil, err
		}
		nn, err := deep.Unmarshal(blob)
		if err != nil {
			return nil, err
		}
		ret = append(ret, nn)
	}
	log.Printf("loaded %d nets with scores %s - %s", len(scores), fmtNum(scores[0].score), fmtNum(scores[len(scores)-1].score))
	return ret, nil
}

type fileScore struct {
	name  string
	score float64
}
