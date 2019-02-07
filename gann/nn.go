package main

import "math"

func makeVector(astat, bstat *charStats) []float64 {
	rateDelta := astat.WinRate() - bstat.WinRate()
	winDelta := astat.AvgWinTime() - bstat.AvgWinTime()
	loseDelta := bstat.AvgLoseTime() - astat.AvgLoseTime()
	bWins := 1 / (1 + math.Pow(10, (astat.Elo-bstat.Elo)/400))
	bWins = 2*bWins - 1
	agames := astat.Wins + astat.Losses
	bgames := bstat.Wins + bstat.Losses
	leastGames := agames
	if bgames < leastGames {
		leastGames = bgames
	}
	return []float64{rateDelta, winDelta, loseDelta, bWins, float64(leastGames)}
}

func (m charStatsMap) Vector(rec *matchRecord) []float64 {
	return makeVector(m[rec.Name[0]], m[rec.Name[1]])
}
