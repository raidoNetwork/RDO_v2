package types

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

func IsStandardTx(tx *types.Transaction) bool {
	switch tx.Type() {
	case common.NormalTxType, common.StakeTxType, common.UnstakeTxType:
		return true
	default:
		return false
	}
}

// IsSystemTx check transaction is created by blockchain self
func IsSystemTx(tx *types.Transaction) bool {
	switch tx.Type() {
	case common.FeeTxType, common.RewardTxType, common.CollapseTxType, common.ValidatorsUnstakeTxType:
		return true
	default:
		return false
	}
}

func PbTxBatchToTyped(batch []*prototype.Transaction) []*types.Transaction {
	res := make([]*types.Transaction, 0, len(batch))

	for _, txpb := range batch {
		res = append(res, types.NewTransaction(txpb))
	}

	return res
}
