package score

import "math"

const (
	maxStrategy = iota
	minStrategy
)

type Scorer struct {
	val uint64
	strategy int8
}

func MinScorer() *Scorer {
	return &Scorer{
		val: math.MaxUint64,
		strategy: minStrategy,
	}
}

func MaxScorer() *Scorer {
	return &Scorer{
		val: 0,
		strategy: maxStrategy,
	}
}

func (s *Scorer) Set(val uint64) {
	switch s.strategy {
	case maxStrategy:
		if val > s.val {
			s.val = val
		}
	case minStrategy:
		if val < s.val {
			s.val = val
		}
	}
}

func (s *Scorer) Get() uint64 {
	return s.val
}
