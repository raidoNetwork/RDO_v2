package generator

import (
	"crypto/ecdsa"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/rpc/api"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

func NewService(chain api.ChainAPI) *Service {
	cfg := params.RaidoConfig()
	stakeAmount := cfg.StakeSlotUnit * cfg.RoiPerRdo

	return &Service{
		chain:       chain,
		stakeAmount: stakeAmount,
		blackHole:   common.HexToAddress(common.BlackHoleAddress).Bytes(),
	}
}

type Service struct {
	chain       api.ChainAPI
	stakeAmount uint64
	blackHole   []byte
}

func (s *Service) GenerateTx(outputs []*prototype.TxOutput, fee uint64, hexKey string) (*prototype.Transaction, error) {
	// get address and private key from hex
	address, key, err := s.getAddress(hexKey)
	if err != nil {
		return nil, err
	}

	return s.createTx(address, key, outputs, fee, common.NormalTxType)
}

func (s *Service) GenerateStakeTx(fee uint64, hexKey string, amount uint64) (*prototype.Transaction, error) {
	if amount%s.stakeAmount != 0 {
		return nil, errors.New("Wrong stake amount given.")
	}

	// get address and private key from hex
	address, key, err := s.getAddress(hexKey)
	if err != nil {
		return nil, err
	}

	outputs := []*prototype.TxOutput{
		types.NewOutput(
			address.Bytes(),
			amount,
			s.blackHole,
		),
	}

	return s.createTx(address, key, outputs, fee, common.StakeTxType)
}

func (s *Service) GenerateUnstakeTx(fee uint64, hexKey string, amount uint64) (*prototype.Transaction, error) {
	if amount%s.stakeAmount != 0 {
		return nil, errors.New("Wrong unstake amount given.")
	}

	// get address and private key from hex
	address, key, err := s.getAddress(hexKey)
	if err != nil {
		return nil, err
	}

	// get stake deposits of address
	utxo, err := s.chain.GetStakeDeposits(address.Hex())
	if err != nil {
		return nil, err
	}

	utxoSize := len(utxo)
	if utxoSize == 0 {
		return nil, errors.New("No stake deposits on your address.")
	}

	var balance uint64

	inputsArr := make([]*prototype.TxInput, 0, len(utxo))
	for _, uo := range utxo {
		inputsArr = append(inputsArr, uo.ToPbInput())
		balance += uo.Amount
	}

	if balance < amount {
		return nil, errors.New("Not enough stake deposits.")
	}

	stakeLeft := balance - amount

	if stakeLeft%s.stakeAmount != 0 {
		return nil, errors.New("Bad stake amount.")
	}

	outputs := []*prototype.TxOutput{
		types.NewOutput(address.Bytes(), amount, nil), // unstake
	}

	if stakeLeft > 0 {
		outputs = append(outputs, types.NewOutput(address.Bytes(), stakeLeft, s.blackHole)) // stake deposits
	}

	nonce, err := s.chain.GetTransactionsCountHex(address.Hex())
	if err != nil {
		return nil, err
	}

	opts := types.TxOptions{
		Num:     nonce + 1,
		Inputs:  inputsArr,
		Outputs: outputs,
		Fee:     fee,
		Data:    []byte{},
		Type:    common.UnstakeTxType,
	}

	// realFee - tx fee
	realFee, _ := types.CountTxFee(opts)

	opts.Outputs[0].Amount -= realFee

	tx, err := types.NewPbTransaction(opts, key)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (s *Service) getAddress(privKey string) (common.Address, *ecdsa.PrivateKey, error) {
	privKey = privKey[2:] // remove 0x part

	// get private key
	key, err := crypto.HexToECDSA(privKey)
	if err != nil {
		return nil, nil, err
	}

	return crypto.PubkeyToAddress(key.PublicKey), key, nil
}

func (s *Service) getBalance(address string) ([]*prototype.TxInput, uint64, error) {
	// find address utxo
	utxo, err := s.chain.FindAllUTxO(address)
	if err != nil {
		return nil, 0, err
	}

	inputsArr := make([]*prototype.TxInput, 0, len(utxo))

	// count balance
	var balance uint64
	for _, uo := range utxo {
		balance += uo.Amount
		inputsArr = append(inputsArr, uo.ToPbInput())
	}

	return inputsArr, balance, nil
}

func (s *Service) genTxStruct(inputs []*prototype.TxInput, outputs []*prototype.TxOutput, fee, balance uint64, address common.Address, typev uint32) (*types.TxOptions, error) {
	var value uint64
	for _, out := range outputs {
		value += out.Amount
	}

	if value >= balance {
		return nil, errors.New("Not enough balance on the wallet.")
	}

	nonce, err := s.chain.GetTransactionsCountHex(address.Hex())
	if err != nil {
		return nil, err
	}

	opts := types.TxOptions{
		Num:     nonce + 1,
		Inputs:  inputs,
		Outputs: outputs,
		Fee:     fee,
		Data:    []byte{},
		Type:    typev,
	}

	// realFee - tx fee, extraFee - tx fee + change output
	realFee, extraFee := types.CountTxFee(opts)

	price := realFee + value
	if price > balance {
		return nil, errors.New("Not enough balance to pay fee.")
	}

	// if address has change
	if balance > price {
		if balance > extraFee+value {
			change := balance - value - extraFee
			opts.Outputs = append(opts.Outputs, types.NewOutput(address.Bytes(), change, nil))
		} else {
			return nil, errors.New("Not enough balance to pay fee.")
		}
	}

	return &opts, err
}

func (s *Service) createTx(address common.Address, key *ecdsa.PrivateKey, outputs []*prototype.TxOutput, fee uint64, typev uint32) (*prototype.Transaction, error) {
	inputsArr, balance, err := s.getBalance(address.Hex())
	if err != nil {
		return nil, err
	}

	if balance == 0 {
		return nil, errors.New("Zero balance.")
	}

	opts, err := s.genTxStruct(inputsArr, outputs, fee, balance, address, typev)
	if err != nil {
		return nil, err
	}

	tx, err := types.NewPbTransaction(*opts, key)
	if err != nil {
		return nil, err
	}

	return tx, nil
}
