package rdochain

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

var ErrZeroFeeAmount = errors.New("Block has no transactions with fee.")

// GetBlockReward returns reward for the current block
func (bc *BlockChain) GetBlockReward() uint64 {
	return bc.cfg.RewardBase
}

// createFeeTx creates fee transaction
func (bc *BlockChain) createFeeTx(txarr []*prototype.Transaction) (*prototype.Transaction, error) {
	var feeAmount uint64

	for _, tx := range txarr {
		feeAmount += tx.GetRealFee()
	}

	if feeAmount == 0 {
		return nil, ErrZeroFeeAmount
	}

	opts := types.TxOptions{
		Outputs: []*prototype.TxOutput{
			types.NewOutput(common.HexToAddress(common.BlackHoleAddress).Bytes(), feeAmount, nil),
		},
		Type: common.FeeTxType,
		Fee:  0,
		Num:  bc.GetBlockCount(),
	}

	ntx, err := types.NewTx(opts, nil)
	if err != nil {
		return nil, err
	}

	return ntx, nil
}
