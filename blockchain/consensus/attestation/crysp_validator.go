package attestation

import (
	"bytes"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
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
	ErrSmallFee = errors.New("Too small fee price.")
)

type CryspValidatorConfig struct {
	SlotTime               time.Duration // SlotTime defines main generator ticker timeout.
	MinFee                 uint64        // MinFee setups minimal transaction fee price.
	StakeUnit              uint64        // StakeUnit roi amount needed for stake.
	LogStat                bool          // LogStat enables time statistic log entries.
	LogDebugStat           bool          // LogDebugStat enable debug log entries
	ValidatorRegistryLimit int           // ValidatorRegistryLimit defines validator slots count
}

func NewCryspValidator(bc consensus.BlockSpecifying, outDB consensus.OutputsReader, stakeValidator consensus.StakeValidator, cfg *CryspValidatorConfig) *CryspValidator {
	v := CryspValidator{
		blockSpecifying: bc,
		outputsReader:   outDB,
		stakeValidator:  stakeValidator,
		cfg:             cfg,
	}

	return &v
}

type CryspValidator struct {
	outputsReader   consensus.OutputsReader
	blockSpecifying consensus.BlockSpecifying
	stakeValidator  consensus.StakeValidator
	cfg             *CryspValidatorConfig
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

	if cv.cfg.LogStat {
		end := time.Since(start)
		log.Infof("ValidateBlock: Count block balance in %s", common.StatFmt(end))
	}

	// check block tx root
	txRoot, err := cv.blockSpecifying.GenTxRoot(block.Transactions)
	if err != nil {
		log.Error("ValidateBlock: error creating tx root.")
		return err
	}

	if !bytes.Equal(txRoot, block.Txroot) {
		return errors.Errorf("Block tx root mismatch. Given: %s. Expected: %s.", common.Encode(block.Txroot), common.Encode(txRoot))
	}

	tstamp := time.Now().UnixNano() + int64(cv.cfg.SlotTime)
	if tstamp < int64(block.Timestamp) {
		return errors.Errorf("Wrong block timestamp: %d. Timestamp with slot time: %d.", block.Timestamp, tstamp)
	}

	start = time.Now()

	// check if block is already exists in the database
	b, err := cv.blockSpecifying.GetBlockByHash(block.Hash)
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
	prevBlock, err := cv.blockSpecifying.GetBlockByHash(block.Parent)
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

// ValidateTransaction validate transaction and return an error if something is wrong
func (cv *CryspValidator) ValidateTransaction(tx *prototype.Transaction) error {
	switch tx.Type {
	case common.NormalTxType:
		fallthrough
	case common.StakeTxType:
		fallthrough
	case common.UnstakeTxType:
		return cv.validateTxInputs(tx)
	default:
		return errors.New("Undefined tx type.")
	}
}

// ValidateTransactionStruct validates transaction balances, signatures and hash. Use only for legacy tx type.
func (cv *CryspValidator) ValidateTransactionStruct(tx *prototype.Transaction) error {
	// if tx has type different from normal return error
	if !common.IsLegacyTx(tx) {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type)
	}

	// check minimal fee value
	if tx.Fee < cv.cfg.MinFee {
		return ErrSmallFee
	}

	// check tx hash
	err := cv.checkHash(tx)
	if err != nil {
		return err
	}

	// Validate that tx has no empty inputs or outputs
	// also check that tx has correct balance
	err = cv.validateTxBalance(tx)
	if err != nil {
		return err
	}

	return nil
}

// validateTxInputs chek that address has given inputs and enough balance.
// If given normal transaction (send coins from one user to another)
// makes sure that all address inputs are spent in this transaction.
func (cv *CryspValidator) validateTxInputs(tx *prototype.Transaction) error {
	if tx.Num == 0 {
		return errors.New("Wrong transaction nonce.")
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
		return errors.Errorf("ValidateTransaction: Inputs size mismatch: real - %d given - %d. Address: %s", utxoSize, inputsSize)
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
		spentOutputsMap[key] = uo.ToInput()
		balance += uo.Amount
	}

	// get sender address
	from := common.BytesToAddress(tx.Inputs[0].Address)

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		return errors.Errorf("Address: %s has balance 0.", from)
	}

	start := time.Now()

	// validate each input
	alreadySpent, err := cv.checkInputsData(tx, spentOutputsMap)

	if cv.cfg.LogStat {
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

	if cv.cfg.LogDebugStat {
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

	if len(tx.Outputs) == 0 || len(tx.Outputs) > cv.cfg.ValidatorRegistryLimit {
		return errors.New("Wrong outputs size.")
	}

	// reward tx num should be equal to the block num
	if tx.Num != block.Num {
		return errors.New("Wrong tx reward num.")
	}

	// get stakers from database
	stakeDeposits, err := cv.outputsReader.FindStakeDeposits()
	if err != nil {
		return err
	}

	targetOutputsLen := len(stakeDeposits)
	if len(tx.Outputs) != targetOutputsLen {
		return errors.New("Wrong tx reward outputs size.")
	}

	receivers := map[string]int{}
	var to string
	for _, uo := range stakeDeposits {
		to = uo.To.Hex()

		if _, exists := receivers[to]; exists {
			receivers[to]++
		} else {
			receivers[to] = 1
		}
	}

	// count reward amount for each staker
	rewardAmount := cv.stakeValidator.GetRewardAmount(targetOutputsLen)

	// check outputs amount and receiver addresses
	for i, out := range tx.Outputs {
		addr := common.BytesToAddress(out.Address)
		addrHex := addr.Hex()

		if _, exists := receivers[addrHex]; exists {
			receivers[addrHex]--
		} else {
			return errors.Errorf("Undefined staker %s", addrHex)
		}

		if out.Amount != rewardAmount {
			return errors.Errorf("Wrong reward amount on output %d. Expect: %d. Real: %d.", i, rewardAmount, out.Amount)
		}
	}

	// check that all receivers got their reward correctly
	for addr, count := range receivers {
		if count != 0 {
			return errors.Errorf("Address %s didn't receive reward.", addr)
		}
	}


	return nil
}

// validateTxBalance validates tx inputs/outputs size
// and check that total balance of all tx is equal to 0.
func (cv *CryspValidator) validateTxBalance(tx *prototype.Transaction) error {
	if len(tx.Inputs) == 0 {
		return errors.Errorf("Empty tx inputs.")
	}

	if len(tx.Outputs) == 0 {
		return errors.Errorf("Empty tx outputs.")
	}

	// verify tx signature
	signer := types.MakeTxSigner("keccak256")
	err := signer.Verify(tx)
	if err != nil {
		return err
	}

	// check that inputs and outputs balance with fee are equal
	var txBalance uint64 = 0
	for _, in := range tx.Inputs {
		txBalance += in.Amount

		if tx.Type == common.UnstakeTxType && in.Amount < cv.cfg.StakeUnit {
			return errors.New("Too low stake amount value.")
		}
	}

	stakeOutputs := 0
	for _, out := range tx.Outputs {
		txBalance -= out.Amount

		// check stake outputs in the stake transaction
		if tx.Type == common.StakeTxType && common.BytesToHash(out.Node).Hex() == common.StakeAddress {
			if out.Amount < cv.cfg.StakeUnit {
				return errors.New("Too small stake amount.")
			}

			stakeOutputs++
		}
	}

	if tx.Type == common.StakeTxType && stakeOutputs == 0 {
		return errors.New("transaction has no stake outputs.")
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

	from := common.BytesToAddress(tx.Inputs[0].Address).Hex()

	var utxo []*types.UTxO
	var err error

	if tx.Type == common.UnstakeTxType {
		utxo, err = cv.outputsReader.FindStakeDepositsOfAddress(from)
	} else {
		// get user inputs from DB
		utxo, err = cv.outputsReader.FindAllUTxO(from)
	}

	if err != nil {
		return nil, errors.Wrap(err, "ValidateTransaction")
	}

	utxoSize := len(utxo)

	if cv.cfg.LogStat {
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