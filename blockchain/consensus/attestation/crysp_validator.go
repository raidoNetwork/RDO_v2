package attestation

import (
	"bytes"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/math"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/utils/hash"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	utypes "github.com/raidoNetwork/RDO_v2/utils/types"
	"github.com/sirupsen/logrus"
)

var (
	log    = logrus.WithField("prefix", "CryspValidator")
	config = params.RaidoConfig()
)

const maxOutputs = 2000

type StakeType int

const (
	NoStake StakeType = iota
	ValidatorStake
	ElectorStake
)

type CryspValidatorConfig struct {
	SlotTime               time.Duration // SlotTime defines main generator ticker timeout.
	RewardBase             uint64        // RewardBase defines reward per validation slot.
	MinFee                 uint64        // MinFee setups minimal transaction fee price.
	StakeUnit              uint64        // StakeUnit roi amount needed for stake.
	EnableMetrics          bool          // EnableMetrics enables time statistic log entries.
	ValidatorRegistryLimit int           // ValidatorRegistryLimit defines validator slots count
	BlockSize              int           // BlockSize defines the maximum size of the block
}

func NewCryspValidator(bc consensus.BlockchainReader, stakeValidator consensus.StakePool, cfg *CryspValidatorConfig) *CryspValidator {
	v := CryspValidator{
		bc:             bc,
		stakeValidator: stakeValidator,
		cfg:            cfg,
	}

	return &v
}

type CryspValidator struct {
	bc             consensus.BlockchainReader
	stakeValidator consensus.StakePool
	cfg            *CryspValidatorConfig
}

// ValidateTransaction validate transaction and return an error if something is wrong
func (cv *CryspValidator) ValidateTransaction(tx *types.Transaction) error {
	switch tx.Type() {
	case common.UnstakeTxType:
		return cv.validateUnstakeTx(tx)
	case common.StakeTxType:
		st, err := cv.checkStakeType(tx)
		if err != nil {
			return err
		}

		if st == ValidatorStake && !cv.stakeValidator.CanValidatorStake(false) {
			return consensus.ErrStakeLimit
		}

		fallthrough
	case common.NormalTxType:
		return cv.validateTxInputs(tx)
	default:
		return consensus.ErrBadTxType
	}
}

// CheckMaxStakers checks whether the limit of
// stakers for a given validator is reached
func (cv *CryspValidator) CheckMaxStakers(tx *types.Transaction, stakers int) error {
	// Look through all outputs; if for any the number
	// of stakers per validator is exceeded, invalidate the transaction
	for _, out := range tx.Outputs() {
		validator := out.Node().Hex()
		if cv.stakeValidator.NumberStakers(validator)+stakers >= config.MaxNumberOfStakers {
			return errors.New(fmt.Sprintf("The number of stakers for validator %s is at its maximum", validator))
		}
	}
	return nil
}

// ValidateTransactionStruct validates transaction balances, signatures and hash. Use only for legacy tx type.
func (cv *CryspValidator) ValidateTransactionStruct(tx *types.Transaction) error {
	// if tx has type different from normal return error
	if !utypes.IsStandardTx(tx) {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type())
	}

	// check minimal fee value
	if tx.FeePrice() < cv.cfg.MinFee {
		return consensus.ErrSmallFee
	}

	// check tx hash
	err := cv.checkHash(tx)
	if err != nil {
		return err
	}

	// check all inputs has the same sender
	from := tx.From()
	for _, input := range tx.Inputs() {
		if !bytes.Equal(from, input.Address()) {
			return consensus.ErrBadInputOwner
		}
	}

	// Validate that tx has no empty inputs or outputs
	// also check that tx has correct balance
	err = cv.validateTxStructBase(tx)
	if err != nil {
		return err
	}

	return nil
}

// validateTxInputs check that address has given inputs and enough balance.
// If given normal transaction (send coins from one user to another)
// makes sure that all address inputs are spent in this transaction.
func (cv *CryspValidator) validateTxInputs(tx *types.Transaction) error {
	if tx.Num() == 0 {
		return consensus.ErrBadNonce
	}

	// get sender address
	from := tx.From()

	// get address nonce
	nonce, err := cv.bc.GetTransactionsCount(from.Bytes())
	if err != nil {
		return err
	}

	if tx.Num() != nonce+1 {
		return consensus.ErrBadNonce
	}

	// get utxo for transaction
	utxo, err := cv.getTxInputsFromDB(tx)
	if err != nil {
		return err
	}

	// count sizes
	utxoSize := len(utxo)

	if utxoSize == 0 {
		return consensus.ErrUtxoSize
	}

	// create spentOutputs map
	spentOutputsMap := map[string]*types.Input{}

	// count balance and create spent map
	var balance uint64
	for _, uo := range utxo {
		input := uo.ToInput()
		inputKey := serialize.GenKeyFromInput(input)

		// fill map with outputs from db
		spentOutputsMap[inputKey] = input
		balance += uo.Amount
	}

	// if balance is equal to zero try to create new transaction
	if balance == 0 {
		log.Debugf("Address %s has zero balance", from)
		return errors.New("Zero balance on the wallet")
	}

	// validate each input
	_, err = cv.checkInputsData(tx, spentOutputsMap)
	if err != nil {
		log.Errorf("ValidateTransaction: Error checking inputs: %s.", err.Error())
		return err
	}

	// //Check that all spent outputs exist
	// for key := range alreadySpent {
	// 	if _, exists := spentOutputsMap[key]; !exists {
	// 		return errors.Errorf("No such utxo of user %s with key %s.", from, key)
	// 	}
	// }

	return nil
}

// validateCollapseTx validate collapse transaction
func (cv *CryspValidator) validateCollapseTx(tx *types.Transaction, block *prototype.Block) error {
	// collapse tx num should be equal to the block num
	if tx.Num() != block.Num {
		return errors.New("Wrong collapse tx num.")
	}

	// validate that tx has no empty inputs or outputs
	// also check that tx has correct balance
	err := cv.validateTxStructBase(tx)
	if err != nil {
		return err
	}

	// collect sender balances
	addrBalance := map[string]uint64{}
	var lastAddr common.Address
	for _, in := range tx.Inputs() {
		key := in.Address().Hex()
		if b, exists := addrBalance[key]; exists {
			balance, overflow := math.Add64(b, in.Amount())
			if overflow {
				return errors.Errorf("Balance overflow for %s", in.Address().Hex())
			}

			addrBalance[key] = balance
		} else {
			addrBalance[key] = in.Amount()
		}

		lastAddr = in.Address()
	}

	// tx must contain one output for each address
	// with amount equal to sum of address inputs
	readedMap := map[string]struct{}{}
	for _, out := range tx.Outputs() {
		key := out.Address().Hex()

		if balance, exists := addrBalance[key]; exists {
			if balance != out.Amount() {
				return errors.New("Address balance inconsistent sum")
			}

			if _, exists := readedMap[key]; exists {
				return errors.New("Address has to many outputs")
			}

			readedMap[key] = struct{}{}
		} else {
			return errors.New("The recipient without sender")
		}
	}

	// get last addr map
	lastAddrOutputs := map[string]struct{}{}
	for _, in := range tx.Inputs() {
		if !bytes.Equal(in.Address(), lastAddr) {
			continue
		}

		inputKey := serialize.GenKeyFromInput(in)
		lastAddrOutputs[inputKey] = struct{}{}
	}

	// get all utxo
	spentOutputsMap := map[string]*types.Input{}
	for addr := range addrBalance {
		utxo, err := cv.bc.FindAllUTxO(addr)
		if err != nil {
			return err
		}

		isLastAddress := lastAddr.Hex() == addr
		for _, uo := range utxo {
			input := uo.ToInput()
			inputKey := serialize.GenKeyFromInput(input)

			// skip not used last address outputs
			if isLastAddress {
				if _, exists := lastAddrOutputs[inputKey]; !exists {
					continue
				}
			}

			spentOutputsMap[inputKey] = input
		}
	}

	// validate each input
	alreadySpent, err := cv.checkInputsData(tx, spentOutputsMap)
	if err != nil {
		log.Errorf("validateCollapseTx: Error checking inputs: %s.", err)
		return err
	}

	//Check that all outputs are spent
	for key := range spentOutputsMap {
		if _, exists := alreadySpent[key]; !exists {
			return errors.Errorf("Unspent output of collapse tx with key %s.", key)
		}
	}

	return nil
}

// validateFeeTx validate fee transaction
func (cv *CryspValidator) validateFeeTx(tx *types.Transaction, block *prototype.Block) error {
	// if tx has type different from fee return error
	if tx.Type() != common.FeeTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type())
	}

	if len(tx.Outputs()) != 1 {
		return errors.New("Wrong tx fee outputs size.")
	}

	// fee tx num should be equal to the block num
	if tx.Num() != block.Num {
		return errors.New("Wrong tx fee num.")
	}

	// check fee tx amount
	var amount uint64 = 0
	for _, pbtx := range block.Transactions {
		amount += pbtx.GetRealFee()
	}

	if amount != tx.Outputs()[0].Amount() {
		return errors.New("Wrong tx fee amount.")
	}

	return nil
}

// validateAwardTx validate award transaction
func (cv *CryspValidator) validateRewardTx(tx *types.Transaction, block *prototype.Block, rewardRecord map[string]uint64) error {
	// if tx has type different from fee return error
	if tx.Type() != common.RewardTxType {
		return errors.Errorf("Transaction has wrong type: %d.", tx.Type())
	}

	// reward tx num should be equal to the block num
	if tx.Num() != block.Num {
		return errors.New("Wrong tx reward num.")
	}

	rewardMap := cv.stakeValidator.GetRewardMap(common.Encode(block.Proposer.Address))

	processed := map[string]struct{}{}
	for _, out := range tx.Outputs() {
		staker := out.Address().Hex()
		if _, exists := processed[staker]; exists {
			return errors.Errorf("Already processed staker %s", staker)
		}

		if rewardMap[staker] != out.Amount() {
			return errors.Errorf("Wrong staker %s reward amount. Given: %d. Expected: %d", staker, out.Amount(), rewardMap[staker])
		}

		processed[staker] = struct{}{}
		rewardRecord[staker] = out.Amount()
	}

	return nil
}

// validateTxStructBase validates tx inputs/outputs size
// verify signature and check that total balance of all tx is equal to 0.
func (cv *CryspValidator) validateTxStructBase(tx *types.Transaction) error {
	if len(tx.Inputs()) == 0 {
		return errors.New("Empty tx inputs.")
	}

	if len(tx.Outputs()) == 0 {
		return errors.New("Empty tx outputs.")
	}

	if len(tx.Inputs()) > maxOutputs {
		return errors.New("Inputs list is too long")
	}

	if len(tx.Outputs()) > maxOutputs {
		return errors.New("Outputs list is too long")
	}

	// verify tx signature
	if tx.Type() != common.CollapseTxType && tx.Type() != common.ValidatorsUnstakeTxType {
		signer := types.MakeTxSigner("keccak256")
		err := signer.Verify(tx.GetTx())
		if err != nil {
			return err
		}
	}

	// If the system has type ValidatorsUnstakeTxType, then it should have
	// inputs with the same node and one output
	if tx.Type() == common.ValidatorsUnstakeTxType {
		// Should have only one output
		if len(tx.Outputs()) != 1 {
			return errors.New("ValidatorsUnstakeTx should have only one output")
		}

		// Checking inputs have the same type.
		// Each address in the inputs should be equal to the address in the output
		node := tx.Inputs()[0].Node().Hex()
		outAddr := tx.Outputs()[0].Address().Hex()
		for _, in := range tx.Inputs() {
			if in.Node().Hex() != node {
				return errors.New("All inputs should have the same node parameter")
			}

			if in.Address().Hex() != outAddr {
				return errors.New("Each input's address should be equal to the output's address")
			}
		}
	}

	// check that inputs and outputs balance with fee are equal
	var inputsBalance uint64
	for _, in := range tx.Inputs() {
		if in.Amount() == 0 {
			return errors.New("Zero amount on input.")
		}

		if tx.Type() == common.UnstakeTxType && in.Amount() < cv.cfg.StakeUnit {
			return consensus.ErrLowStakeAmount
		}

		var overflow bool
		inputsBalance, overflow = math.Add64(in.Amount(), inputsBalance)
		if overflow {
			return errors.New("Inputs sum amount overflow.")
		}
	}

	stakeOutputs := 0
	var outputsBalance uint64
	for _, out := range tx.Outputs() {
		if out.Amount() == 0 {
			return errors.Errorf("Zero amount on output.")
		}

		// check stake outputs in the stake transaction
		if tx.Type() == common.StakeTxType {
			node := out.Node().Hex()
			if len(out.Node()) == common.AddressLength {
				if (node == common.BlackHoleAddress && out.Amount()%cv.cfg.StakeUnit != 0) || out.Amount() == 0 {
					return errors.Wrap(consensus.ErrLowStakeAmount, "Stake error")
				}

				stakeOutputs++
			}
		}

		var overflow bool
		outputsBalance, overflow = math.Add64(out.Amount(), outputsBalance)
		if overflow {
			return errors.New("Outputs sum amount overflow.")
		}
	}

	if tx.Type() == common.StakeTxType && stakeOutputs == 0 {
		return errors.New("transaction has no stake outputs.")
	}

	var overflow bool
	outputsBalance, overflow = math.Add64(outputsBalance, tx.Fee())
	if overflow {
		return errors.New("Outputs sum amount overflow.")
	}

	if inputsBalance != outputsBalance {
		diff, underflow := math.Sub64(inputsBalance, outputsBalance)
		if outputsBalance > inputsBalance {
			diff, _ = math.Sub64(outputsBalance, inputsBalance)
		}

		return errors.Errorf("tx balance is inconsistent. Mismatch is %d. Underflow %v", diff, underflow)
	}

	return nil
}

// getTxInputsFromDB get all unspent outptus of tx sender
func (cv *CryspValidator) getTxInputsFromDB(tx *types.Transaction) ([]*types.UTxO, error) {
	start := time.Now()

	from := tx.From().Hex()

	var utxo []*types.UTxO
	var err error

	if tx.Type() == common.UnstakeTxType {
		utxo, err = cv.bc.FindStakeDepositsOfAddress(from, "all")
	} else {
		// get user inputs from DB
		utxo, err = cv.bc.FindAllUTxO(from)
	}

	if err != nil {
		return nil, errors.Wrap(err, "ValidateTransaction")
	}

	utxoSize := len(utxo)

	if cv.cfg.EnableMetrics {
		end := time.Since(start)
		log.Debugf("ValidateTransaction: Read all UTxO of user %s Count: %d Time: %s", from, utxoSize, common.StatFmt(end))
	}

	return utxo, nil
}

// checkInputsData check tx inputs with database inputs
func (cv *CryspValidator) checkInputsData(tx *types.Transaction, spentOutputsMap map[string]*types.Input) (map[string]struct{}, error) {
	// get sender address
	from := tx.From().Hex()
	txHash := tx.Hash().Hex()
	alreadySpent := map[string]struct{}{}

	// Inputs verification
	for _, in := range tx.Inputs() {
		key := serialize.GenKeyFromInput(in)

		dbInput, exists := spentOutputsMap[key]
		if !exists {
			return nil, errors.Errorf("User %s gave undefined output with key: %s on tx %s.", from, key, txHash)
		}

		if _, exists := alreadySpent[key]; exists {
			return nil, errors.Errorf("User %s try to spend output twice with key: %s on tx %s", from, key, txHash)
		}

		if !bytes.Equal(dbInput.Hash(), in.Hash()) {
			return nil, errors.Errorf("Hash mismatch with key %s. Given %s. Expected %s. Tx %s", key, in.Hash().Hex(), dbInput.Hash().Hex(), txHash)
		}

		if in.Index() != dbInput.Index() {
			return nil, errors.Errorf("Index mismatch with key %s. Given %d. Expected %d. Tx %s", key, in.Index(), dbInput.Index(), txHash)
		}

		if in.Amount() != dbInput.Amount() {
			return nil, errors.Errorf("Amount mismatch with key: %s. Given %d. Expected %d. Tx %s", key, in.Amount(), dbInput.Amount(), txHash)
		}

		if !bytes.Equal(dbInput.Node(), in.Node()) {
			return nil, errors.Errorf("Node mismatch with key %s. Given %s. Expected %s. Tx %s", key, in.Node().Hex(), dbInput.Node().Hex(), txHash)
		}

		// mark output as already spent
		alreadySpent[key] = struct{}{}
	}

	return alreadySpent, nil
}

// checkHash check that tx hash is calculated correctly
func (cv *CryspValidator) checkHash(tx *types.Transaction) error {
	genHash, err := hash.TxHash(tx.GetTx())
	if err != nil {
		return err
	}

	if !bytes.Equal(genHash.Bytes(), tx.Hash()) {
		return errors.Errorf("Transaction has wrong hash. Given: %s. Expected: %s.", tx.Hash().Hex(), genHash.Hex())
	}

	return nil
}

// validateUnstakeTx check unstake tx
func (cv *CryspValidator) validateUnstakeTx(tx *types.Transaction) error {
	stakeNode := ""
	for _, in := range tx.Inputs() {
		node := in.Node().Hex()
		if stakeNode == "" {
			stakeNode = node
		} else if stakeNode != node {
			return errors.Errorf("Unstake from different nodes is not allowed. Given %s, %s", stakeNode, node)
		}
	}

	// validate unstake outputs are valid
	for _, out := range tx.Outputs() {
		if out.Node().Hex() == common.BlackHoleAddress && (out.Amount()%cv.cfg.StakeUnit) != 0 {
			return errors.New("Wrong stake output amount.")
		}
	}

	return cv.validateTxInputs(tx)
}

// checkSystemUnstakeTx verifies SystemUnstake transactions
func (cv *CryspValidator) validateSystemUnstakeTx(tx *types.Transaction, block *prototype.Block) error {
	// Num of SystemUnstake should be equal to the block num
	if tx.Num() != block.Num {
		return errors.New("Wrong system unstake tx num")
	}
	err := cv.validateTxStructBase(tx)
	if err != nil {
		return err
	}

	var sumIn uint64
	var overflow bool
	for _, in := range tx.Inputs() {
		sumIn, overflow = math.Add64(sumIn, in.Amount())
		if overflow {
			return errors.Errorf("Balance overflow for %s", in.Address().Hex())
		}
	}

	// We have one output per staker. We want to make sure
	// that the sum of inputs for a particular staker is
	// equal to the amount in the output
	sumOut := tx.Outputs()[0].Amount()
	if sumOut != sumIn {
		return errors.Errorf("Balance inconsistency in transaction: %s", tx.Hash().Hex())
	}

	// Finally, check that the utxos correspond to the inputs
	// get all utxo
	spentOutputsMap := map[string]*types.Input{}
	addr := tx.Outputs()[0].Address().Hex()
	deposits, err := cv.bc.FindStakeDepositsOfAddress(addr, "all")
	if err != nil {
		return err
	}

	for _, uo := range deposits {
		input := uo.ToInput()
		inputKey := serialize.GenKeyFromInput(input)
		spentOutputsMap[inputKey] = input
	}

	// validate each input
	alreadySpent, err := cv.checkInputsData(tx, spentOutputsMap)
	if err != nil {
		log.Errorf("ValidateSystemUnstakeTx: Error checking inputs: %s.", err)
		return err
	}
	if len(alreadySpent) != len(tx.Inputs()) {
		err = errors.New("ValidateSystemUnstakeTx: Inputs do not correspond to deposits")
		log.Error(err)
		return err
	}
	return nil
}

// checkStakeType check stake tx type
func (cv *CryspValidator) checkStakeType(tx *types.Transaction) (StakeType, error) {
	hasStaking := false
	hasValidatorStaking := false
	var validator string
	for _, out := range tx.Outputs() {
		if len(out.Node()) == common.AddressLength {
			if out.Node().Hex() == common.BlackHoleAddress {
				hasValidatorStaking = true
			} else {
				if !cv.stakeValidator.HasValidator(out.Node().Hex()) {
					return NoStake, errors.New("Validator not found")
				}

				if validator == "" {
					validator = out.Node().Hex()
				} else if validator != out.Node().Hex() {
					return NoStake, errors.New("Different validators stake in one tx")
				}
			}

			hasStaking = true
		}
	}

	if hasValidatorStaking && validator != "" {
		return NoStake, errors.New("Different stake types in one tx")
	}

	// Invalidate the stake tx if the number of stakers per validator is exceeded
	if err := cv.IsMaxStakers(tx); err != nil {
		return NoStake, err
	}

	if !hasStaking {
		return NoStake, nil
	}

	if hasValidatorStaking {
		return ValidatorStake, nil
	} else {
		return ElectorStake, nil
	}
}

func (cv *CryspValidator) IsMaxStakers(tx *types.Transaction) error {
	// Look through all outputs; if for any the number
	// of stakers per validator is exceeded, invalidate the transaction
	for _, out := range tx.Outputs() {
		validator := out.Node().Hex()
		if cv.stakeValidator.NumberStakers(validator) >= config.MaxNumberOfStakers {
			return errors.New(fmt.Sprintf("The number of stakers for validator %s is at its maximum", validator))
		}
	}
	return nil
}
