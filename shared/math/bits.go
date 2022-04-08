package math

import "math/bits"

func Add64(a, b uint64) (uint64, bool){
	sum, overflow := bits.Add64(a, b, 0)
	return sum, overflow > 0
}

func Sub64(a, b uint64) (uint64, bool){
	sum, borrow := bits.Sub64(a, b, 0)
	return sum, borrow > 0
}
