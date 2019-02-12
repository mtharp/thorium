package main

import deep "github.com/patrikeh/go-deep"

// vectors
const (
	predResponseSize = 2
	betResponseSize  = 1
)

// genetic algorithm
const (
	population  = 256
	eliteSelect = 4
	mutateMin   = 0.5
	mutateMax   = 5.0
	termSlope   = 0.0002
	termStride  = 5
	termMinGen  = 30
	metaMaxGen  = 30
	metaPop     = 32
)

// betting limits
const (
	mmScale        = 1.0 / 20
	alwaysAllIn    = 5000
	maxAllIn       = 100e3
	maxBet         = 256e3
	defaultBailout = 425
	tournBailout   = 1000 + defaultBailout
)

// consensus
const (
	consensusNets = 3
	consensusMeta = 0
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
