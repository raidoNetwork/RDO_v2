package common

import (
	"fmt"
	"time"
)

// Outputs types
const (
	UnspentTxO = 0
	SpentTxO   = 1
)

// Transaction types
const (
	NormalTxType  = 1
	GenesisTxType = 2
	FeeTxType = 3
	RewardTxType = 4
	StakeTxType   = 5
	UnstakeTxType = 6
)

const (
	StakeAddress     = "0x0000000000000000000000000000000000000000000000000000000000000000"
	BlackHoleAddress = "0x0000000000000000000000000000000000000000"
)

const (
	SlotTime = 1 * time.Second

	AccountNum = 10
	TxLimitPerBlock = 4
)

// StatFmt parse time.Duration to the needed string format
func StatFmt(d time.Duration) string {
	return fmt.Sprintf("%d μs", int64(d/time.Microsecond))
}
