package rdochain

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"math"
)

const (
	initReward     uint64 = 10000000000 // 10 000 000 000
	rewardInterval uint64 = 300000      // each 300 000 blocks
)

// GetReward returns reward for the current block
func (bc *BlockChain) GetReward() uint64 {
	return bc.GetRewardForBlock(bc.GetBlockCount())
}

// GetRewardForBlock count reward for block with given number
func (bc *BlockChain) GetRewardForBlock(num uint64) uint64 {
	var award = initReward
	blocksKoef := num / rewardInterval
	divider := math.Pow(2.0, float64(blocksKoef))

	award /= uint64(divider)

	if bc.fullStatFlag {
		log.Debugf("Koef: %d. Num: %d. Divider %f. Reward: %d.", blocksKoef, num, divider, award)
	}

	return award
}

// createFeeTx creates fee transaction
func (bc *BlockChain) createFeeTx(txarr []*prototype.Transaction) (*prototype.Transaction, error) {
	var feeAmount uint64 = 0

	for _, tx := range txarr {
		feeAmount += tx.GetRealFee()
	}

	opts := types.TxOptions{
		Outputs: []*prototype.TxOutput{
			types.NewOutput(nodeAddress.Bytes(), feeAmount),
		},
		Type: common.FeeTxType,
	}

	ntx, err := types.NewTx(opts)
	if err != nil {
		return nil, err
	}

	// FeeTx num should be equal
	// to the current block num
	ntx.Num = bc.blockNum

	return ntx, nil
}

// createRewardTx create reward transaction
func (bc *BlockChain) createRewardTx() (*prototype.Transaction, error) {
	reward := bc.GetReward()

	opts := types.TxOptions{
		Outputs: []*prototype.TxOutput{
			types.NewOutput(nodeAddress.Bytes(), reward),
		},
		Type: common.RewardTxType,
	}

	log.Warnf("Get reward %d.", reward)

	ntx, err := types.NewTx(opts)
	if err != nil {
		return nil, err
	}

	// RewardTx num should be equal
	// to the current block num
	ntx.Num = bc.blockNum

	return ntx, nil
}
