package score

import (
	"math"
	"sync"
)

const (
	maxStrategy = iota
	minStrategy
)

type Scorer struct {
	val uint64
	strategy int8
	initialized bool

	sync.Mutex
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
	s.Lock()
	defer s.Unlock()

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
	s.Lock()
	defer s.Unlock()

	return s.val
}

func (s *Scorer) Initialize() {
	s.Lock()
	defer s.Unlock()

	s.initialized = true
}

func (s *Scorer) Initialized() bool {
	s.Lock()
	defer s.Unlock()

	return s.initialized
}
