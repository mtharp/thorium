package main

import deep "github.com/patrikeh/go-deep"

// vectors
const (
	predResponseSize = 2
	betResponseSize  = 1

	betVectorWithBank = 0
	betVectorSize     = predResponseSize + 3 + betVectorWithBank
)

// genetic algorithm
const (
	population  = 256
	eliteSelect = 4
	mutateMin   = 0.5
	mutateMax   = 5.0
	termSlope   = 0.01
	termStride  = 3
	termMinGen  = 10
	termMetaGen = 30
	metaPop     = 32
)

// betting limits
const (
	mmScale        = 1.0 / 20
	exhibScale     = 1.0 / 60
	alwaysAllIn    = 5000
	maxAllIn       = 100e3
	maxBet         = 256e3
	defaultBailout = 425
	tournBailout   = 1000 + defaultBailout
)

var (
	predCfg = &deep.Config{
		Layout:     []int{5, 3, predResponseSize},
		Activation: deep.ActivationSigmoid,
		Mode:       deep.ModeRegression,
		Weight:     deep.NewNormal(1.0, 0.0),
		Bias:       true,
	}
	betCfg = &deep.Config{
		Inputs:     betVectorSize,
		Layout:     []int{5, 4, betResponseSize},
		Activation: deep.ActivationSigmoid,
		Mode:       deep.ModeRegression,
		Weight:     deep.NewNormal(1.0, 0.0),
	}
)
