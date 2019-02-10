package main

import (
	"fmt"
	"log"
	"sort"
	"strings"
)

type wager float64

func wagerFromVector(o []float64) wager {
	switch len(o) {
	case 2:
		j, k := o[0], o[1]
		if j < 0 && k < 0 {
			return 0
		} else if k > 0 {
			return wager(k)
		} else {
			return wager(-j)
		}
	case 1:
		return wager(o[0])
	default:
		panic("invalid output size")
	}
}

func (w wager) Size() float64 {
	if w < 0 {
		return float64(-w)
	}
	return float64(w)
}

func (w wager) PredictB() bool {
	return w > 0
}

type wagerList []wager

func (l wagerList) Less(i, j int) bool {
	return l[i] < l[j]
}

func (l wagerList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l wagerList) Len() int {
	return len(l)
}

func (l wagerList) Consensus() wager {
	sort.Sort(l)
	if len(l)%2 == 0 {
		panic("consensus set must have odd length")
	}
	i := len(l) / 2
	if true {
		var w []string
		for j, y := range l {
			s := fmt.Sprintf("%d", int(100*y))
			if i == j {
				s = "[" + s + "]"
			}
			w = append(w, s)
		}
		log.Println("consensus:", strings.Join(w, " "))
	}
	return l[i]
}
