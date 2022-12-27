package common

import (
	"fmt"
	"time"

	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

// Transaction types
const (
	NormalTxType        = 1
	GenesisTxType       = 2
	FeeTxType           = 3
	RewardTxType        = 4
	StakeTxType         = 5
	UnstakeTxType       = 6
	CollapseTxType      = 7
	SystemUnstakeTxType = 8
)

const (
	BlackHoleAddress = "0x0000000000000000000000000000000000000000"
)

var (
	MillisecondsBuckets   = []float64{250, 500, 1000, 1500, 2000, 4000, 8000, 16000}
	KVMillisecondsBuckets = []float64{50, 100, 150, 200, 250, 400, 500, 1000, 2000}
)

// StatFmt parse time.Duration to the needed string format
func StatFmt(d time.Duration) string {
	return fmt.Sprintf("%d μs", d.Microseconds())
}

// IsLegacyTx check transaction type and return true if transaction is legacy:
// send coins from one address to another or stake.
func IsLegacyTx(tx *prototype.Transaction) bool {
	switch tx.Type {
	case NormalTxType:
		fallthrough
	case StakeTxType:
		fallthrough
	case UnstakeTxType:
		return true
	default:
		return false
	}
}

// IsSystemTx check transaction is created by blockchain self
func IsSystemTx(tx *prototype.Transaction) bool {
	switch tx.Type {
	case GenesisTxType:
		fallthrough
	case FeeTxType:
		fallthrough
	case RewardTxType:
		fallthrough
	case SystemUnstakeTxType:
		fallthrough
	case CollapseTxType:
		return true
	default:
		return false
	}
}

// HasInputs check transactions should have inputs
func HasInputs(tx *prototype.Transaction) bool {
	return IsLegacyTx(tx) || tx.Type == CollapseTxType || tx.Type == SystemUnstakeTxType
}
