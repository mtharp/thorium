package main

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
