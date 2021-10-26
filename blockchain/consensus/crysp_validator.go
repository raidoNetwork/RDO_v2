package consensus

import (
	"bytes"
	"encoding/hex"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"rdo_draft/blockchain/core/rdochain"
	"rdo_draft/blockchain/db"
	"rdo_draft/proto/prototype"
	"rdo_draft/shared/common"
	"rdo_draft/shared/types"
	"strconv"
	"time"
)

var (
	log = logrus.WithField("prefix", "CryspValidator")

	ErrUtxoSize = errors.New("UTxO count is 0.")
)

func NewCryspValidator(bc *rdochain.BlockChain, outDB db.OutputManager, stat bool, expStat bool) *CryspValidator {
	v := CryspValidator{
		bc:          bc,
		outDB:       outDB,
		statFlag:    stat,
		expStatFlag: expStat,
	}

	return &v
}

type CryspValidator struct {
	outDB db.OutputManager     // sqlite utxo hash
	bc    *rdochain.BlockChain // blockchain

	statFlag    bool
	expStatFlag bool
}

// ValidateTransaction validate transaction and return an error if something is wrong
func (cv *CryspValidator) ValidateTransaction(tx *prototype.Transaction) error {
	switch tx.Type {
	case common.NormalTxType:
		return cv.validateNormalTx(tx)
	case common.GenesisTxType:
		log.Warnf("Validate genesis tx %s.", hex.EncodeToString(tx.Hash))
		return nil
	case common.FeeTxType:
		return cv.validateFeeTx(tx)
	case common.RewardTxType:
		return cv.validateAwardTx(tx)
	default:
		return errors.New("Undefined tx type.")
	}
}

// validateNormalTx check that transaction is valid
func (cv *CryspValidator) validateNormalTx(tx *prototype.Transaction) error {
	// if tx has type different from normal return error
	if tx.Type != common.NormalTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type)
	}

	start := time.Now()

	// Validate that tx has not empty inputs or outputs
	// also check that tx has correct balance
	err := cv.validateTxBalance(tx)
	if err != nil {
		return err
	}

	if cv.expStatFlag {
		end := time.Since(start)
		log.Infof("ValidateTransaction: Check Tx balance in %s.", common.StatFmt(end))
	}

	// get utxo for transaction
	utxo, err := cv.getUTxO(tx)

	// count sizes
	utxoSize := len(utxo)
	inputsSize := len(tx.Inputs)

	if utxoSize != inputsSize {
		return errors.Errorf("ValidateTransaction: Inputs size mismatch: real - %d given - %d.", utxoSize, inputsSize)
	}

	if utxoSize == 0 {
		return ErrUtxoSize
	}

	// create spentOutputs map
	spentOutputsMap := map[string]*prototype.TxInput{}

	// count balance and create spent map
	var balance uint64
	var key, hash, indexStr string
	for _, uo := range utxo {
		hash = hex.EncodeToString(uo.Hash)
		indexStr = strconv.Itoa(int(uo.Index))
		key = hash + "_" + indexStr

		// fill map with outputs from db
		spentOutputsMap[key] = uo.ToInput(nil)
		balance += uo.Amount
	}

	// get sender address
	from := hex.EncodeToString(tx.Inputs[0].Address)

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		return errors.Errorf("Account %s has balance 0.", from)
	}

	start = time.Now()

	// validate each input
	alreadySpent, err := cv.checkInputsData(tx, spentOutputsMap)

	if cv.statFlag {
		end := time.Since(start)
		log.Infof("ValidateTransaction: Inputs verification. Count: %d. Time: %s.", inputsSize, common.StatFmt(end))
	}

	start = time.Now()

	//Check that all outputs are spent
	for _, isSpent := range alreadySpent {
		if isSpent != 1 {
			return errors.Errorf("Unspent output of user %s with key %s.", from, key)
		}
	}

	if cv.expStatFlag {
		end := time.Since(start)
		log.Infof("ValidateTransaction: Verify all inputs are lock for spent in %s.", common.StatFmt(end))
	}

	log.Warnf("Validated tx %s", hex.EncodeToString(tx.Hash))

	return nil
}

// validateFeeTx validate fee transaction
func (cv *CryspValidator) validateFeeTx(tx *prototype.Transaction) error {
	// if tx has type different from fee return error
	if tx.Type != common.FeeTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type)
	}

	return nil
}

// validateAwardTx validate award transaction
func (cv *CryspValidator) validateAwardTx(tx *prototype.Transaction) error {
	// if tx has type different from fee return error
	if tx.Type != common.RewardTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type)
	}

	if len(tx.Outputs) != 1 {
		return errors.New("Wrong tx reward outputs size.")
	}

	// Find reward for block number equal to tx num.
	// AwardTx num is always equal to the block number.
	if tx.Outputs[0].Amount != cv.bc.GetRewardForBlock(tx.Num) {
		return errors.New("Wrong block reward given.")
	}

	return nil
}

// validateTxBalance validates tx inputs/outputs size
// and check that total balance of all tx is equal to 0.
func (cv *CryspValidator) validateTxBalance(tx *prototype.Transaction) error {
	// FeeTx has no inputs another types must have any inputs.
	if tx.Type == common.NormalTxType && len(tx.Inputs) == 0 {
		return errors.Errorf("Empty tx inputs.")
	}

	if len(tx.Outputs) == 0 {
		return errors.Errorf("Empty tx outputs.")
	}

	// check that inputs and outputs balance with fee are equal
	var txBalance uint64 = 0
	for _, in := range tx.Inputs {
		txBalance += in.Amount
	}

	for _, out := range tx.Outputs {
		txBalance -= out.Amount
	}

	txBalance -= tx.GetRealFee()

	if txBalance != 0 {
		return errors.Errorf("Tx balance is inconsistent. Mismatch is %d.", txBalance)
	}

	return nil
}

// getUTxO get all unspent outptus of tx sender
func (cv *CryspValidator) getUTxO(tx *prototype.Transaction) ([]*types.UTxO, error) {
	start := time.Now()

	from := hex.EncodeToString(tx.Inputs[0].Address)

	// get user inputs from DB
	utxo, err := cv.outDB.FindAllUTxO(from)
	if err != nil {
		return nil, errors.Wrap(err, "ValidateTransaction")
	}

	utxoSize := len(utxo)

	if cv.statFlag {
		end := time.Since(start)
		log.Infof("ValidateTransaction: Read all UTxO of user %s Count: %d Time: %s", from, utxoSize, common.StatFmt(end))
	}

	return utxo, nil
}

// checkInputsData check tx inputs with database inputs
func (cv *CryspValidator) checkInputsData(tx *prototype.Transaction, spentOutputsMap map[string]*prototype.TxInput) (map[string]int, error) {
	// get sender address
	from := hex.EncodeToString(tx.Inputs[0].Address)
	alreadySpent := map[string]int{}

	var hash, indexStr, key string

	// Inputs verification
	for _, in := range tx.Inputs {
		hash = hex.EncodeToString(in.Hash)
		indexStr = strconv.Itoa(int(in.Index))
		key = hash + "_" + indexStr

		dbInput, exists := spentOutputsMap[key]
		if !exists {
			return nil, errors.Errorf("User %s gave undefined output with key: %s.", from, key)
		}

		if alreadySpent[key] == 1 {
			return nil, errors.Errorf("User %s try to spend output twice with key: %s.", from, key)
		}

		if !bytes.Equal(dbInput.Hash, in.Hash) {
			return nil, errors.Errorf("Hash mismatch with key %s. Given %s. Expected %s.", key, hash, hex.EncodeToString(dbInput.Hash))
		}

		if in.Index != dbInput.Index {
			return nil, errors.Errorf("Index mismatch with key %s. Given %d. Expected %d.", key, in.Index, dbInput.Index)
		}

		if in.Amount != dbInput.Amount {
			return nil, errors.Errorf("Amount mismatch with key: %s. Given %d. Expected %d.", key, in.Amount, dbInput.Amount)
		}

		// mark output as already spent
		alreadySpent[key] = 1
	}

	return alreadySpent, nil
}

// ValidateBlock validates given block.
func (cv *CryspValidator) ValidateBlock(block *prototype.Block) error {
	// check that block has no double in outputs and inputs
	inputExists := map[string]string{}
	outputExists := map[string]string{}

	start := time.Now()

	var blockBalance uint64 = 0

	for txIndex, tx := range block.Transactions {
		// skip reward tx
		if tx.Type == common.RewardTxType {
			if len(tx.Outputs) != 1 {
				return errors.New("Wrong tx reward outputs size.")
			}

			if tx.Outputs[0].Amount != cv.bc.GetReward() {
				return errors.New("Wrong block reward given.")
			}

			continue
		}

		// check inputs
		for _, in := range tx.Inputs {
			key := hex.EncodeToString(in.Hash) + "_" + strconv.Itoa(int(in.Index))

			hash, exists := inputExists[key]
			if exists {
				curHash := hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(txIndex)
				return errors.Errorf("Block #%d has double input in tx %s with key %s. Bad tx: %s.", block.Num, hash, key, curHash)
			}

			inputExists[key] = hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(txIndex)
			blockBalance += in.Amount
		}

		// check outputs
		for outIndex, out := range tx.Outputs {
			key := hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(outIndex)

			hash, exists := outputExists[key]
			if exists {
				curHash := hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(txIndex)
				return errors.Errorf("Block #%d has double output in tx %s with key %s. Bad tx: %s.", block.Num, hash, key, curHash)
			}

			outputExists[key] = hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(txIndex)
			blockBalance -= out.Amount
		}
	}

	if cv.statFlag {
		end := time.Since(start)
		log.Infof("Check block tx doubles in %s", common.StatFmt(end))
	}

	if blockBalance != 0 {
		return errors.New("Wrong block balance.")
	}

	// check block tx root
	txRoot, err := cv.bc.GenTxRoot(block.Transactions)
	if err != nil {
		log.Error("BlockChain.ValidateBlock: error creating tx root.")
		return err
	}

	if !bytes.Equal(txRoot, block.Txroot) {
		return errors.Errorf("Block tx root mismatch. Given: %s. Expected: %s.", hex.EncodeToString(block.Txroot), hex.EncodeToString(txRoot))
	}

	tstamp := time.Now().UnixNano() + int64(common.SlotTime)
	if tstamp < int64(block.Timestamp) {
		return errors.Errorf("Wrong block timestamp: %d. Timestamp with slot time: %d.", block.Timestamp, tstamp)
	}

	start = time.Now()

	// check if block exists
	b, err := cv.bc.GetBlockByNum(int(block.Num))
	if err != nil {
		return errors.New("Error reading block from database.")
	}

	if cv.statFlag {
		end := time.Since(start)
		log.Infof("Get block by num in %s", common.StatFmt(end))
	}

	if b != nil {
		return errors.Errorf("Block #%d is already exists in blockchain!", block.Num)
	}

	start = time.Now()

	// find prevBlock
	prevBlockNum := int(block.Num) - 1
	prevBlock, err := cv.bc.GetBlockByNum(prevBlockNum)
	if err != nil {
		return errors.New("Error reading block from database.")
	}

	if cv.statFlag {
		end := time.Since(start)
		log.Infof("Check prev block in %s", common.StatFmt(end))
	}

	if prevBlock == nil {
		// check if prevBlock is genesis
		if prevBlockNum == 0 {
			// return Genesis
		} else {
			return errors.Errorf("Previous Block #%d for given block #%d is not exists.", block.Num-1, block.Num)
		}
	}

	return nil
}
