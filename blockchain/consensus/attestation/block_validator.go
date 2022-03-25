package attestation

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/utils/hash"
	"strconv"
	"time"
)

// checkBlockBalance count block inputs and outputs sum and check that all inputs in block are unique.
func (cv *CryspValidator) checkBlockBalance(block *prototype.Block) error {
	// check that block has no double in outputs and inputs
	inputExists := map[string]string{}

	var blockInputsBalance, blockOutputsBalance uint64
	for txIndex, tx := range block.Transactions {
		// skip collapse tx
		if tx.Type == common.CollapseTxType {
			err := cv.validateCollapseTx(tx, block)
			if err != nil {
				return errors.Wrap(err, "Error validation CollapseTx")
			}
			continue
		}

		// validate reward tx and skip it
		// because reward tx brings inconsistency in block balance
		if tx.Type == common.RewardTxType {
			err := cv.validateRewardTx(tx, block)
			if err != nil {
				return errors.Wrap(err, "Error validation RewardTx")
			}

			continue
		}

		//validate fee tx
		if tx.Type == common.FeeTxType {
			err := cv.validateFeeTx(tx, block)
			if err != nil {
				return errors.Wrap(err, "Error validation FeeTx")
			}
		}

		txHash := common.BytesToHash(tx.Hash)

		// check inputs
		for _, in := range tx.Inputs {
			inHash := common.BytesToHash(in.Hash)
			key := inHash.Hex() + "_" + strconv.Itoa(int(in.Index))
			txHashIndex := txHash.Hex() + "_" + strconv.Itoa(txIndex)

			hash, exists := inputExists[key]
			if exists {
				curHash := txHashIndex

				log.Errorf("Saved tx: %s", hash)
				log.Errorf("Double spend tx: %s", curHash)
				return errors.Errorf("Block #%d has double input with key %s", block.Num, key)
			}

			inputExists[key] = txHashIndex
			blockInputsBalance += in.Amount
		}

		// check outputs
		for _, out := range tx.Outputs {
			blockOutputsBalance += out.Amount
		}
	}

	if blockInputsBalance != blockOutputsBalance {
		return errors.New("Wrong block balance.")
	}

	return nil
}

// ValidateBlock validate block and return an error if something is wrong
func (cv *CryspValidator) ValidateBlock(block *prototype.Block) error {
	start := time.Now()

	// check that block has total balance equal to zero
	// check that inputs of block don't repeat
	err := cv.checkBlockBalance(block)
	if err != nil {
		return err
	}

	if cv.cfg.LogStat {
		end := time.Since(start)
		log.Infof("ValidateBlock: Count block balance in %s", common.StatFmt(end))
	}

	// check block tx root
	txRoot := hash.GenTxRoot(block.Transactions)
	if !bytes.Equal(txRoot, block.Txroot) {
		return errors.Errorf("Block tx root mismatch. Given: %s. Expected: %s.", common.Encode(block.Txroot), common.Encode(txRoot))
	}

	tstamp := time.Now().UnixNano() + int64(cv.cfg.SlotTime)
	if tstamp < int64(block.Timestamp) {
		return errors.Errorf("Wrong block timestamp: %d. Timestamp with slot time: %d.", block.Timestamp, tstamp)
	}

	start = time.Now()

	// check if block is already exists in the database
	b, err := cv.bc.GetBlockByHash(block.Hash)
	if err != nil {
		return errors.New("Error reading block from database.")
	}

	if cv.cfg.LogStat {
		end := time.Since(start)
		log.Infof("ValidateBlock: Get block by hash in %s", common.StatFmt(end))
	}

	if b != nil {
		return errors.Errorf("ValidateBlock: Block #%d is already exists in blockchain!", block.Num)
	}

	start = time.Now()

	// find prevBlock
	prevBlock, err := cv.bc.GetBlockByHash(block.Parent)
	if err != nil {
		return errors.Errorf("ValidateBlock: Error reading previous block from database. Hash: %s.", common.BytesToHash(block.Parent))
	}

	if cv.cfg.LogStat {
		end := time.Since(start)
		log.Infof("ValidateBlock: Get prev block in %s", common.StatFmt(end))
	}

	if prevBlock == nil {
		return errors.Errorf("ValidateBlock: Previous Block #%d for given block #%d is not exists.", block.Num-1, block.Num)
	}

	if prevBlock.Timestamp >= block.Timestamp {
		return errors.Errorf("ValidateBlock: Timestamp is too small. Previous: %d. Current: %d.", prevBlock.Timestamp, block.Timestamp)
	}

	// TODO check block approvers and slashers

	return nil

}
