package main

import deep "github.com/patrikeh/go-deep"

// vectors
const betResponseSize = 1

// genetic algorithm
const (
	population  = 512
	eliteSelect = 4
	mutateMin   = 0.2
	mutateMax   = 5.0
	termSlope   = 0.001
	termStride  = 20
	termMinGen  = 20
)

// betting limits
const (
	mmScale        = 1 / 2.0
	trnScale       = 2.0
	alwaysAllIn    = 5000
	maxAllIn       = 100e3
	maxBet         = 256e3
	defaultBailout = 450
	tournBailout   = 1000 + defaultBailout
)

var (
	betCfg = &deep.Config{
		Inputs:     betVectorSize,
		Layout:     []int{5, 3, betResponseSize},
		Activation: deep.ActivationSigmoid,
		Mode:       deep.ModeRegression,
		Weight:     deep.NewNormal(1.0, 0.0),
	}
)
