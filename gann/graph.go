package main

type matchup struct {
	Wins, Losses float64
}

func (m matchup) Add(win bool) matchup {
	if win {
		m.Wins++
	} else {
		m.Losses++
	}
	return m
}

func (m matchup) Score() float64 {
	if m.Wins == 0 && m.Losses == 0 {
		return 0
	}
	return (m.Wins - m.Losses) / (m.Wins + m.Losses)
}

func (s *charStats) AddMatchup(opponent string, win bool) {
	if s.matchups == nil {
		s.matchups = make(map[string]matchup)
	}
	s.matchups[opponent] = s.matchups[opponent].Add(win)
}

func (cm charStatsMap) ABXY(a, x string) float64 {
	var sum, count float64
	for y, ym := range cm[a].matchups {
		for b, bm := range cm[y].matchups {
			if b == a {
				continue
			}
			xm, ok := cm[b].matchups[x]
			if ok {
				sum += ym.Score() - bm.Score() + xm.Score()
				count++
			}
		}
	}
	if count == 0 {
		return 0
	}
	return sum / count
}
