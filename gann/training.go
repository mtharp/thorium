package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"

	deep "github.com/patrikeh/go-deep"
)

type evalFunc func(nn *deep.Neural, debug bool) float64
type shufFunc func()

type score struct {
	nn    *deep.Neural
	score float64
}

type scoreList []score

func (l scoreList) Len() int {
	return len(l)
}

func (l scoreList) Less(i, j int) bool {
	// highest first
	return l[i].score > l[j].score
}

func (l scoreList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func train(ncfg *deep.Config, eval evalFunc, shuf shufFunc, rng *rand.Rand, pop []*deep.Neural) (*deep.Neural, float64) {
	meta := pop != nil
	if !meta {
		pop = make([]*deep.Neural, population)
		for i := 0; i < population; i++ {
			pop[i] = deep.NewNeural(ncfg)
		}
	}
	var prevScores [10]float64
	var lastScore float64
	for gen := 0; ; gen++ {
		if gen != 0 && shuf != nil {
			if gen >= metaMaxGen {
				break
			}
			shuf()
		}
		scores := make(scoreList, 0, len(pop))
		sch := make(chan score, len(pop))
		for _, nn := range pop {
			nn := nn
			go func() {
				nscore := score{nn, eval(nn, false)}
				if math.IsNaN(nscore.score) {
					nscore.score = -1e6
				}
				sch <- nscore
			}()
		}
		for i := 0; i < len(pop); i++ {
			scores = append(scores, <-sch)
		}
		sort.Sort(scores)
		if false {
			for _, nscore := range scores {
				log.Printf("%f %p", nscore.score, nscore.nn)
			}
		}
		lastScore := scores[0].score
		if prevScores[0] != 0 && shuf == nil {
			if lastScore < prevScores[0] {
				log.Fatalln("score regressed -- data race?")
			}
			pprev := prevScores[termStride-1]
			minGen := termMinGen
			if gen >= minGen-1 && (lastScore-pprev)/pprev < termSlope {
				log.Printf("terminating after %d generations - score %s", gen+1, fmtNum(lastScore))
				return scores[0].nn, lastScore
			}
		}
		copy(prevScores[1:], prevScores[:])
		prevScores[0] = lastScore
		nn2 := make([]*deep.Neural, eliteSelect, population)

		for i := 0; i < eliteSelect; i++ {
			nn2[i] = scores[i].nn
		}
		for len(nn2) < population {
			p1 := selectParent(rng, scores, nil)
			p2 := selectParent(rng, scores, p1)
			// increase mutation sharply towards the end of the population
			j := math.Pow(float64(len(nn2)-eliteSelect)/float64(len(pop)-eliteSelect), 3)
			sigma := mutateMin + j*(mutateMax-mutateMin)
			nn2 = append(nn2, cross(rng, sigma, p1, p2))
		}
		pop = nn2
		log.Printf("%d %s", gen, fmtNum(lastScore))
	}
	if meta {
		lastScore = 0
		for _, score := range prevScores {
			lastScore += score
		}
		lastScore /= float64(len(prevScores))
		log.Printf("meta-training score: %s", fmtNum(lastScore))
	}
	return pop[0], lastScore
}

func selectParent(rng *rand.Rand, scores scoreList, otherParent *deep.Neural) *deep.Neural {
	// pick a random individual but heavily prefer the highest scoring ones
	k := int(float64(len(scores)-1) * math.Pow(rng.Float64(), 3))
	if k > len(scores)-1 {
		k = len(scores) - 1
	}
	p := scores[k].nn
	if p == otherParent {
		if k == 0 {
			return scores[1].nn
		}
		return scores[k-1].nn
	}
	return scores[k].nn
}

func cross(rng *rand.Rand, sigma float64, p1, p2 *deep.Neural) *deep.Neural {
	if false {
		log.Printf("P1 %s", fmtNet(p1))
		log.Printf("P2 %s", fmtNet(p2))
	}
	child := deep.NewNeural(p1.Config)
	for i := range p1.Layers {
		l1 := p1.Layers[i]
		l2 := p2.Layers[i]
		for j := range l1.Neurons {
			n1 := l1.Neurons[j]
			n2 := l2.Neurons[j]
			for k := range n1.In {
				w1 := n1.In[k].Weight
				w2 := n2.In[k].Weight
				m := rng.NormFloat64() * sigma
				w := ((w1 + w2) + m*(w1-w2)) / 2
				child.Layers[i].Neurons[j].In[k].Weight = w
			}
		}
	}
	if false {
		log.Printf("Ch %s", fmtNet(child))
	}
	return child
}

func fmtNet(nn *deep.Neural) string {
	var words []string
	for _, l := range nn.Layers {
		for _, n := range l.Neurons {
			for _, i := range n.In {
				words = append(words, fmt.Sprintf("%+.3f", i.Weight))
			}
		}
	}
	return strings.Join(words, " ")
}

func fmtNum(n float64) string {
	switch {
	case n > 1e12:
		return fmt.Sprintf("%.2ft", n/1e12)
	case n > 1e9:
		return fmt.Sprintf("%.2fb", n/1e9)
	case n > 1e6:
		return fmt.Sprintf("%.2fm", n/1e6)
	case n > 1e3:
		return fmt.Sprintf("%.2fk", n/1e3)
	default:
		return fmt.Sprintf("%.2f", n)
	}
}
