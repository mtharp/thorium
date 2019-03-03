package main

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	deep "github.com/patrikeh/go-deep"
)

func netFromFiles(dirname string) (*deep.Neural, error) {
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
	if len(scores) == 0 {
		return nil, errors.New("no files in " + dirname)
	}
	blob, err := ioutil.ReadFile(filepath.Join(dirname, scores[0].name))
	if err != nil {
		return nil, err
	}
	defer log.Printf("loaded %s", scores[0].name)
	return deep.Unmarshal(blob)
}

type fileScore struct {
	name  string
	score float64
}
