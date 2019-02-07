package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"sort"
	"strings"

	deep "github.com/patrikeh/go-deep"
)

const (
	generations = 99999999
	population  = 144
	eliteSelect = 4
	mutateMin   = 0.5
	mutateMax   = 10.0
	superMutant = 20.0
)

type evalFunc func(nn *deep.Neural, debug io.Writer) float64

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

func train(ncfg *deep.Config, eval evalFunc, rng *rand.Rand) *deep.Neural {
	pop := make([]*deep.Neural, population)
	for i := range pop {
		pop[i] = deep.NewNeural(ncfg)
	}
	mutateMaxGen := mutateMax
	var genScores []float64
	for gen := 0; gen < generations; gen++ {
		scores := make(scoreList, 0, population)
		sch := make(chan score, population)
		for _, nn := range pop {
			nn := nn
			go func() {
				nscore := score{nn, eval(nn, nil)}
				if math.IsNaN(nscore.score) {
					nscore.score = -1e6
				}
				sch <- nscore
			}()
		}
		for i := 0; i < population; i++ {
			scores = append(scores, <-sch)
		}
		sort.Sort(scores)
		if false {
			for _, nscore := range scores {
				log.Printf("%f %p", nscore.score, nscore.nn)
			}
		}
		nn2 := make([]*deep.Neural, eliteSelect, population)
		for i := 0; i < eliteSelect; i++ {
			nn2[i] = scores[i].nn
		}
		genScores = append(genScores, scores[0].score)
		for len(nn2) < population {
			// pick a random pair but heavily prefer the highest scoring ones
			k := int(float64(len(scores)-2) * math.Pow(rng.Float64(), 3))
			if k > population-2 {
				k = population - 2
			}
			p1, p2 := scores[k].nn, scores[k+1].nn
			sigma := mutateMin
			if len(genScores) > 5 && genScores[len(genScores)-4] == scores[0].score {
				mutateMaxGen++
			}
			if len(nn2) > population*3/4 {
				sigma = mutateMaxGen
			}
			if len(nn2) == population-1 {
				sigma *= superMutant
			}
			nn2 = append(nn2, cross(rng, sigma, p1, p2))
		}
		pop = nn2
		log.Printf("%d %f", gen, scores[0].score)
		blob, err := nn2[0].Marshal()
		if err != nil {
			log.Fatalln("error:", err)
		}
		if err := ioutil.WriteFile("bettor.dat", blob, 0644); err != nil {
			log.Fatalln("error:", err)
		}
	}
	return pop[0]
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
