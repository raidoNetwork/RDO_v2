package common

import (
	"fmt"
	"time"
)

const (
	UnspentTxO = 0
	SpentTxO   = 1

	NormalTxType  = 1
	GenesisTxType = 2
)

func StatFmt(d time.Duration) string {
	return fmt.Sprintf("%d μs", int64(d/time.Microsecond))
}
