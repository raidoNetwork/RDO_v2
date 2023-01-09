package attestation

import (
	"bytes"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/rdochain"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/math"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/utils/hash"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	vtypes "github.com/raidoNetwork/RDO_v2/validator/types"
)

var (
	ErrReadingBlock = errors.New("Error reading block from database")
)

const failedTxLimitPercent = 50

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

func (cv *CryspValidator) ValidateGenesis(block *prototype.Block) error {
	genesis := cv.bc.GetGenesis()
	ghash := common.BytesToHash(genesis.Hash)
	if !bytes.Equal(block.Hash, ghash.Bytes()) {
		return errors.Errorf("Genesis hash mismatch. Expected: %s. Given: %s", ghash.Hex(), common.Encode(block.Hash))
	}

	err := cv.validateBlockHeader(block)
	if err != nil {
		return err
	}

	for i, tx := range block.Transactions {
		if !bytes.Equal(tx.Hash, genesis.Transactions[i].Hash) {
			panic("Bad Genesis")
		}
	}

	return nil
}

func (cv *CryspValidator) validateBlockHeader(block *prototype.Block) error {
	// check block tx root
	txRoot := hash.GenTxRoot(block.Transactions)
	if !bytes.Equal(txRoot, block.Txroot) {
		return errors.Errorf("Block tx root mismatch. Given: %s. Expected: %s.", common.Encode(block.Txroot), common.Encode(txRoot))
	}

	tstamp := time.Now().UnixNano() + int64(cv.cfg.SlotTime)
	if tstamp < int64(block.Timestamp) {
		return errors.Errorf("Wrong block timestamp: %d. Timestamp with slot time: %d.", block.Timestamp, tstamp)
	}

	return nil
}

// ValidateBlock validate block and return an error if something is wrong
func (cv *CryspValidator) ValidateBlock(block *prototype.Block, countSign bool) ([]*types.Transaction, error) {
	log.Debugf("Validate block #%d", block.Num)

	start := time.Now()

	// check that block has total balance equal to zero
	// check that inputs of block don't repeat
	err := cv.checkBlockBalance(block)
	if err != nil {
		return nil, err
	}

	if cv.cfg.EnableMetrics {
		end := time.Since(start)
		log.Debugf("ValidateBlock: Count block balance in %s", common.StatFmt(end))
	}

	// validate block base fields
	err = cv.validateBlockHeader(block)
	if err != nil {
		return nil, err
	}

	blockHash := hash.BlockHash(block.Num, block.Slot, block.Version, block.Parent, block.Txroot, block.Timestamp, block.Proposer.Address)
	if !bytes.Equal(blockHash, block.Hash) {
		return nil, errors.Errorf("Bad block hash given. Expected: %s. Given: %s", common.Encode(blockHash), common.Encode(block.Hash))
	}

	err = cv.verifyBlockSign(block, block.Proposer)
	if err != nil {
		return nil, errors.New("Wrong block proposer signature")
	}

	// check if block is already exists in the database
	b, err := cv.bc.GetBlockByHash(block.Hash)
	if err != nil {
		log.Debugf("Error reading block by hash %s", err)
		return nil, ErrReadingBlock
	}

	if b != nil {
		log.Debugf("ValidateBlock: GetBlockByHash: Block #%d is already exists in blockchain!", block.Num)
		return nil, consensus.ErrKnownBlock
	} else {
		b, err = cv.bc.GetBlockByNum(block.Num)
		if err != nil && !errors.Is(err, rdochain.ErrNotForgedBlock) {
			log.Debugf("Error reading block by num %s", err)
			return nil, ErrReadingBlock
		}

		if b != nil {
			log.Debugf("ValidateBlock: GetBlockByNum: Block #%d is already exists in blockchain!", block.Num)
			return nil, consensus.ErrKnownBlock
		}
	}

	start = time.Now()

	// find prevBlock
	prevBlock, err := cv.bc.GetBlockByHash(block.Parent)
	if err != nil {
		return nil, errors.Errorf("ValidateBlock: Error reading previous block from database. Hash: %s.", common.BytesToHash(block.Parent))
	}

	if cv.cfg.EnableMetrics {
		end := time.Since(start)
		log.Debugf("ValidateBlock: Get prev block in %s", common.StatFmt(end))
	}

	if prevBlock == nil {
		return nil, errors.Errorf("ValidateBlock: Previous Block #%d for given block #%d is not exists.", block.Num-1, block.Num)
	}

	if prevBlock.Timestamp >= block.Timestamp {
		return nil, errors.Errorf("ValidateBlock: Timestamp is too small. Previous: %d. Current: %d.", prevBlock.Timestamp, block.Timestamp)
	}

	blockNumDiff := block.Num - prevBlock.Num
	if blockNumDiff > 1 {
		return nil, errors.Errorf("ValidateBlock: Too big block num difference. Need: 1. Current: %d", blockNumDiff)
	}

	if countSign {
		approversCount := cv.countValidSigns(block, block.Approvers, vtypes.Approve)
		slashersCount := cv.countValidSigns(block, block.Slashers, vtypes.Reject)

		err = consensus.IsEnoughVotes(approversCount, slashersCount)
		if err != nil {
			return nil, errors.Wrap(err, "Block voting error")
		}

		log.Infof("Approvers %d, slashers %d", approversCount, slashersCount)
	}

	failedTx, err := cv.verifyTransactions(block)
	if err != nil {
		return failedTx, errors.Wrap(err, "Error verifying transactions")
	}

	return nil, nil

}

func (cv *CryspValidator) verifyTransactions(block *prototype.Block) ([]*types.Transaction, error) {
	failedTx := make([]*types.Transaction, 0)
	standardTxCount := 0
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
			case common.ValidatorsUnstakeTxType:
				err = cv.validateSystemUnstakeTx(tx, block)
				txType = "SystemUnstakeTx"
			}

			if err != nil {
				return nil, errors.Wrap(err, "Error validating "+txType)
			}

			continue
		}

		standardTxCount++
		err := cv.ValidateTransactionStruct(tx)
		if err != nil {
			log.Error(err)
			failedTx = append(failedTx, tx)
			tx.SetStatus(types.TxFailed)
			continue
		}

		err = cv.ValidateTransaction(tx)
		if err != nil {
			log.Error(err)
			failedTx = append(failedTx, tx)
			tx.SetStatus(types.TxFailed)
			continue
		}

		tx.SetStatus(types.TxSuccess)
	}

	if math.IsGEPercentLimit(len(failedTx), standardTxCount, failedTxLimitPercent) {
		return failedTx, errors.New("too many failed tx")
	}

	return nil, nil
}

func (cv *CryspValidator) verifyBlockSign(block *prototype.Block, sign *prototype.Sign) error {
	header := types.NewHeader(block)
	return types.GetBlockSigner().Verify(header, sign)
}

func (cv *CryspValidator) countValidSigns(block *prototype.Block, signatures []*prototype.Sign, attestationType vtypes.AttestationType) int {
	validCount := 0
	header := types.NewHeader(block)
	for _, sign := range signatures {
		err := vtypes.VerifyBlockSign(header, attestationType, sign)
		if err != nil {
			continue
		}

		validCount += 1
	}

	return validCount
}
