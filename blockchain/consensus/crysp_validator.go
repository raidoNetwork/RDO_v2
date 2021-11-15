package consensus

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/hasher"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

var (
	log = logrus.WithField("prefix", "CryspValidator")

	ErrUtxoSize = errors.New("UTxO count is 0.")
)

func NewCryspValidator(bc BlockSpecifying, outDB BalanceReader, stat bool, expStat bool) *CryspValidator {
	v := CryspValidator{
		bs:          bc,
		br:          outDB,
		statFlag:    stat,
		expStatFlag: expStat,
	}

	return &v
}

type CryspValidator struct {
	br BalanceReader
	bs BlockSpecifying

	statFlag    bool
	expStatFlag bool
}

func (cv *CryspValidator) checkBlockBalance(block *prototype.Block) error {
	// check that block has no double in outputs and inputs
	inputExists := map[string]string{}

	var blockBalance uint64 = 0
	for txIndex, tx := range block.Transactions {
		// validate reward tx and skip it
		// because reward tx brings inconsistency in block balance
		if tx.Type == common.RewardTxType {
			err := cv.validateRewardTx(tx, block)
			if err != nil {
				return err
			}

			continue
		}

		//validate fee tx
		if tx.Type == common.FeeTxType {
			err := cv.validateFeeTx(tx, block)
			if err != nil {
				return err
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
				return errors.Errorf("Block #%d has double input with key %s.\n\tCur tx: %s.\n\tBad tx: %s.", block.Num, key, hash, curHash)
			}

			inputExists[key] = txHashIndex
			blockBalance += in.Amount
		}

		// check outputs
		for _, out := range tx.Outputs {
			blockBalance -= out.Amount
		}
	}

	if blockBalance != 0 {
		return errors.New("Wrong block balance.")
	}

	return nil
}

// ValidateBlock validate block and return an error if something is wrong
func (cv *CryspValidator) ValidateBlock(block *prototype.Block) error {
	start := time.Now()

	// check that block has total balance equal to zero
	// check that inputs and outputs of bock doesn't repeat
	err := cv.checkBlockBalance(block)
	if err != nil {
		return err
	}

	if cv.statFlag {
		end := time.Since(start)
		log.Infof("ValidateBlock: Count block balance in %s", common.StatFmt(end))
	}

	// check block tx root
	txRoot, err := cv.bs.GenTxRoot(block.Transactions)
	if err != nil {
		log.Error("ValidateBlock: error creating tx root.")
		return err
	}

	if !bytes.Equal(txRoot, block.Txroot) {
		return errors.Errorf("Block tx root mismatch. Given: %s. Expected: %s.", common.Encode(block.Txroot), common.Encode(txRoot))
	}

	tstamp := time.Now().UnixNano() + int64(common.SlotTime)
	if tstamp < int64(block.Timestamp) {
		return errors.Errorf("Wrong block timestamp: %d. Timestamp with slot time: %d.", block.Timestamp, tstamp)
	}

	start = time.Now()

	// check if block is already exists in the database
	b, err := cv.bs.GetBlockByHash(block.Hash)
	if err != nil {
		return errors.New("Error reading block from database.")
	}

	if cv.statFlag {
		end := time.Since(start)
		log.Infof("ValidateBlock: Get block by num in %s", common.StatFmt(end))
	}

	if b != nil {
		return errors.Errorf("ValidateBlock: Block #%d is already exists in blockchain!", block.Num)
	}

	start = time.Now()

	// find prevBlock
	prevBlock, err := cv.bs.GetBlockByHash(block.Parent)
	if err != nil {
		return errors.New("ValidateBlock: Error reading previous block from database.")
	}

	if cv.statFlag {
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

// ValidateTransaction validate transaction and return an error if something is wrong
func (cv *CryspValidator) ValidateTransaction(tx *prototype.Transaction) error {
	switch tx.Type {
	case common.NormalTxType:
		return cv.validateNormalTx(tx)
	default:
		return errors.New("Undefined tx type.")
	}
}

// ValidateTransactionData validates transaction balances, signatures and hash. Use only for normal tx type.
func (cv *CryspValidator) ValidateTransactionData(tx *prototype.Transaction) error {
	// if tx has type different from normal return error
	if tx.Type != common.NormalTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type)
	}

	// check tx hash
	err := cv.checkHash(tx)
	if err != nil {
		return err
	}

	start := time.Now()

	// Validate that tx has not empty inputs or outputs
	// also check that tx has correct balance
	err = cv.validateTxBalance(tx)
	if err != nil {
		return err
	}

	if cv.expStatFlag {
		end := time.Since(start)
		log.Infof("ValidateTransaction: Check tx balance in %s.", common.StatFmt(end))
	}

	return nil
}

// validateNormalTx check that transaction is valid
func (cv *CryspValidator) validateNormalTx(tx *prototype.Transaction) error {
	// if tx has type different from normal return error
	if tx.Type != common.NormalTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type)
	}

	// get utxo for transaction
	utxo, err := cv.getUTxO(tx)
	if err != nil {
		return err
	}

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
	var key, indexStr string
	for _, uo := range utxo {
		indexStr = strconv.Itoa(int(uo.Index))
		key = uo.Hash.Hex() + "_" + indexStr

		// fill map with outputs from db
		spentOutputsMap[key] = uo.ToInput(nil)
		balance += uo.Amount
	}

	// get sender address
	from := common.BytesToAddress(tx.Inputs[0].Address)

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		return errors.Errorf("Account %s has balance 0.", from)
	}

	start := time.Now()

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

	log.Warnf("Validated tx %s", common.BytesToHash(tx.Hash))

	return nil
}

// validateFeeTx validate fee transaction
func (cv *CryspValidator) validateFeeTx(tx *prototype.Transaction, block *prototype.Block) error {
	// if tx has type different from fee return error
	if tx.Type != common.FeeTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type)
	}

	if len(tx.Outputs) != 1 {
		return errors.New("Wrong tx fee outputs size.")
	}

	// fee tx num should be equal to the block num
	if tx.Num != block.Num {
		return errors.New("Wrong tx fee num.")
	}

	// check fee tx amount
	var amount uint64 = 0
	for _, txi := range block.Transactions {
		amount += txi.GetRealFee()
	}

	if amount != tx.Outputs[0].Amount {
		return errors.New("Wrong tx fee amount.")
	}

	return nil
}

// validateAwardTx validate award transaction
func (cv *CryspValidator) validateRewardTx(tx *prototype.Transaction, block *prototype.Block) error {
	// if tx has type different from fee return error
	if tx.Type != common.RewardTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type)
	}

	if len(tx.Outputs) != 1 {
		return errors.New("Wrong tx reward outputs size.")
	}

	// reward tx num should be equal to the block num
	if tx.Num != block.Num {
		return errors.New("Wrong tx reward num.")
	}

	// Find reward for block number equal to tx num.
	// AwardTx num is always equal to the block number.
	if tx.Outputs[0].Amount != cv.bs.GetRewardForBlock(block.Num) {
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

	// verify tx signature
	if tx.Type == common.NormalTxType {
		signer := types.MakeTxSigner("keccak256")
		err := signer.Verify(tx)
		if err != nil {
			return err
		}
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
		return errors.Errorf("tx balance is inconsistent. Mismatch is %d.", txBalance)
	}

	return nil
}

// getUTxO get all unspent outptus of tx sender
func (cv *CryspValidator) getUTxO(tx *prototype.Transaction) ([]*types.UTxO, error) {
	start := time.Now()

	from := common.BytesToAddress(tx.Inputs[0].Address)

	// get user inputs from DB
	utxo, err := cv.br.FindAllUTxO(from.Hex())
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
	from := common.Encode(tx.Inputs[0].Address)
	alreadySpent := map[string]int{}

	var hash, indexStr, key string

	// Inputs verification
	for _, in := range tx.Inputs {
		hash = common.Encode(in.Hash)
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
			return nil, errors.Errorf("Hash mismatch with key %s. Given %s. Expected %s.", key, hash, common.Encode(dbInput.Hash))
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

// checkHash check that tx hash is calculated correctly
func (cv *CryspValidator) checkHash(tx *prototype.Transaction) error {
	genHash, err := hasher.TxHash(tx)
	if err != nil {
		return err
	}

	if !bytes.Equal(genHash.Bytes(), tx.Hash) {
		return errors.Errorf("Transaction has wrong hash. Given: %s. Expected: %s.", common.Encode(tx.Hash), genHash.Hex())
	}

	return nil
}
