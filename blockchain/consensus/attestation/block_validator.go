package attestation

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/utils/hash"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	"strconv"
	"time"
)

// checkBlockBalance count block inputs and outputs sum and check that all inputs in block are unique.
func (cv *CryspValidator) checkBlockBalance(block *prototype.Block) error {
	// check that block has no double in outputs and inputs
	inputExists := map[string]string{}

	var blockInputsBalance, blockOutputsBalance uint64
	for txIndex, txpb := range block.Transactions {
		txHash := common.BytesToHash(txpb.Hash).Hex()

		if txpb.Type == common.RewardTxType {
			continue
		}

		// check inputs
		for _, in := range txpb.Inputs {
			key := serialize.GenKeyFromPbInput(in)
			txHashIndex := txHash + "_" + strconv.Itoa(txIndex)

			inputHash, exists := inputExists[key]
			if exists {
				curHash := txHashIndex

				log.Errorf("Saved txpb: %s", inputHash)
				log.Errorf("Double spend txpb: %s", curHash)
				return errors.Errorf("Block #%d has double input with key %s", block.Num, key)
			}

			inputExists[key] = txHashIndex
			blockInputsBalance += in.Amount
		}

		// check outputs
		for _, out := range txpb.Outputs {
			blockOutputsBalance += out.Amount
		}
	}

	if blockInputsBalance != blockOutputsBalance {
		return errors.New("Wrong block balance.")
	}

	return nil
}

// ValidateBlock validate block and return an error if something is wrong
func (cv *CryspValidator) ValidateBlock(block *prototype.Block) (*types.Transaction, error) {
	start := time.Now()

	// check that block has total balance equal to zero
	// check that inputs of block don't repeat
	err := cv.checkBlockBalance(block)
	if err != nil {
		return nil, err
	}

	if cv.cfg.EnableMetrics {
		end := time.Since(start)
		log.Infof("ValidateBlock: Count block balance in %s", common.StatFmt(end))
	}

	// check block tx root
	txRoot := hash.GenTxRoot(block.Transactions)
	if !bytes.Equal(txRoot, block.Txroot) {
		return nil, errors.Errorf("Block tx root mismatch. Given: %s. Expected: %s.", common.Encode(block.Txroot), common.Encode(txRoot))
	}

	tstamp := time.Now().UnixNano() + int64(cv.cfg.SlotTime)
	if tstamp < int64(block.Timestamp) {
		return nil, errors.Errorf("Wrong block timestamp: %d. Timestamp with slot time: %d.", block.Timestamp, tstamp)
	}

	err = cv.verifyBlockSign(block, block.Proposer)
	if err != nil {
		return nil, errors.New("Wrong block proposer signature")
	}

	start = time.Now()

	// check if block is already exists in the database
	b, err := cv.bc.GetBlockByHash(block.Hash)
	if err != nil {
		return nil, errors.New("Error reading block from database.")
	}

	if cv.cfg.EnableMetrics {
		end := time.Since(start)
		log.Infof("ValidateBlock: Get block by hash in %s", common.StatFmt(end))
	}

	if b != nil {
		return nil, errors.Errorf("ValidateBlock: Block #%d is already exists in blockchain!", block.Num)
	}

	start = time.Now()

	// find prevBlock
	prevBlock, err := cv.bc.GetBlockByHash(block.Parent)
	if err != nil {
		return nil, errors.Errorf("ValidateBlock: Error reading previous block from database. Hash: %s.", common.BytesToHash(block.Parent))
	}

	if cv.cfg.EnableMetrics {
		end := time.Since(start)
		log.Infof("ValidateBlock: Get prev block in %s", common.StatFmt(end))
	}

	if prevBlock == nil {
		return nil, errors.Errorf("ValidateBlock: Previous Block #%d for given block #%d is not exists.", block.Num-1, block.Num)
	}

	if prevBlock.Timestamp >= block.Timestamp {
		return nil, errors.Errorf("ValidateBlock: Timestamp is too small. Previous: %d. Current: %d.", prevBlock.Timestamp, block.Timestamp)
	}

	approversCount := cv.countValidSigns(block, block.Approvers)
	slashersCount := cv.countValidSigns(block, block.Slashers)

	log.Infof("Approvers %d, slashers %d", approversCount, slashersCount)

	failedTx, err := cv.verifyTransactions(block)
	if err != nil {
		return failedTx, errors.Wrap(err, "Error verify transactions")
	}

	return nil, nil

}

func (cv *CryspValidator) verifyTransactions(block *prototype.Block) (*types.Transaction, error) {
	for _, txpb := range block.Transactions {
		tx := types.NewTransaction(txpb)

		if common.IsSystemTx(txpb) {
			var txType string
			var err error
			switch tx.Type() {
			case common.CollapseTxType:
				err = cv.validateCollapseTx(tx, block)
				txType = "CollapseTx"
			case common.FeeTxType:
				err = cv.validateFeeTx(tx, block)
				txType = "FeeTx"
			case common.RewardTxType:
				err = cv.validateRewardTx(tx, block)
				txType = "RewardTx"
			}

			if err != nil {
				return nil, errors.Wrap(err, "Error validation " + txType)
			}

			continue
		}

		err := cv.commitTx(tx)
		if err != nil {
			return tx, err
		}
	}

	return nil, nil
}

func (cv *CryspValidator) commitTx(tx *types.Transaction) error {
	err := cv.ValidateTransactionStruct(tx)
	if err != nil {
		return err
	}

	return cv.ValidateTransaction(tx)
}

func (cv *CryspValidator) verifyBlockSign(block *prototype.Block, sign *prototype.Sign) error {
	header := types.NewHeader(block)
	return types.GetBlockSigner().Verify(header, sign)
}

func (cv *CryspValidator) countValidSigns(block *prototype.Block, signatures []*prototype.Sign) int {
	validCount := 0
	for _, sign := range signatures {
		err := cv.verifyBlockSign(block, sign)
		if err != nil {
			continue
		}

		validCount += 1
	}

	return validCount
}