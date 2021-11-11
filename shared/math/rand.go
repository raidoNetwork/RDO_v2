package math

import (
	"math/rand"
	"time"
)

func RandUint64(min, max uint64) uint64 {
	rand.Seed(time.Now().UnixNano())
	rnd := rand.Intn(100) + 1

	res := uint64(rnd)*(max-min)/100 + min

	return res
}
