package staking

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"sync"
	"unicode/utf8"
)

var log = logrus.WithField("prefix", "StakePool")

type ValidatorStakeData struct {
	SlotsFilled     int
	CumulativeStake uint64
	Electors        map[string]uint64 // Staker => StakedAmount
	SelfStake       uint64
}

type StakingPool struct {
	validators map[string]*ValidatorStakeData
	cumulativeStake uint64
	rewardPerBlock uint64
	stakeAmountPerSlot uint64
	slotsLimit int
	slotsFilled int
	slotsReserved int
	blockchain consensus.BlockchainReader

	mu sync.Mutex
}

func (p *StakingPool) Init() error {
	deposits, err := p.blockchain.FindStakeDeposits()
	if err != nil {
		return err
	}

	for _, uo := range deposits {
		if uo.Node.Hex() == common.BlackHoleAddress {
			err = p.registerValidatorStake(uo.To.Hex(), uo.Amount)
		} else {
			err = p.registerElectorStake(uo.From.Hex(), uo.To.Hex(), uo.Amount)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (p *StakingPool) registerValidatorStake(validator string, amount uint64) error {
	empty := p.getEmptySlots()
	if empty == 0 {
		return errors.New("Validator slots limit is reached.")
	}

	if empty < 0 {
		return errors.New("Validator slots inconsistent.")
	}

	slotsCount := int(amount / p.stakeAmountPerSlot)
	if empty < slotsCount {
		return errors.Errorf("Can't feel all slots. Empty: %d. Given: %d.", empty, slotsCount)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.validators[validator]; !exists {
		p.validators[validator] = &ValidatorStakeData{
			Electors: map[string]uint64{},
		}
	}

	validatorData := p.validators[validator]
	validatorData.CumulativeStake += amount
	validatorData.SelfStake += amount
	validatorData.SlotsFilled += slotsCount

	p.slotsFilled += slotsCount
	p.cumulativeStake += amount

	return nil
}

func (p *StakingPool) cancelValidatorStake(validator string, amount uint64) error  {
	slotsCount := int(amount / p.stakeAmountPerSlot)
	if slotsCount == 0 {
		return errors.Errorf("Bad unstake amount given: %d.", amount)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.validators[validator]; !exists {
		return errors.New("Validator not found")
	}

	validatorData := p.validators[validator]

	if validatorData.SlotsFilled == slotsCount {
		delete(p.validators, validator)
	} else {
		validatorData.SelfStake -= amount
		validatorData.SlotsFilled -= slotsCount
	}

	p.cumulativeStake -= amount
	p.slotsFilled -= slotsCount

	return nil
}

func (p *StakingPool) registerElectorStake(elector, validator string, amount uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.validators[validator]; !exists {
		log.Debugf("Not found validator %s with elector %s", validator, elector)
		return errors.New("Not found validator")
	}

	validatorData := p.validators[validator]
	validatorData.CumulativeStake += amount
	validatorData.Electors[elector] += amount

	p.cumulativeStake += amount

	return nil
}

func (p *StakingPool) cancelElectorStake(elector, validator string, amount uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.validators[validator]; !exists {
		log.Debugf("Not found validator %s with elector %s", validator, elector)
		return errors.New("Not found validator")
	}

	validatorData := p.validators[validator]
	if _, exists := validatorData.Electors[elector]; !exists {
		return errors.New("Not found elector")
	}

	if validatorData.Electors[elector] == amount {
		delete(validatorData.Electors, elector)
	} else {
		validatorData.Electors[elector] -= amount
	}

	validatorData.CumulativeStake -= amount
	p.cumulativeStake -= amount

	return nil
}

func (p *StakingPool) getEmptySlots() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.slotsLimit - p.slotsFilled
}

func (p *StakingPool) filledSlots(includeReserved bool) int {
	slots := p.slotsFilled
	if includeReserved {
		slots += p.slotsReserved
	}

	return slots
}

func (p *StakingPool) CanValidatorStake(includeReserved bool) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.filledSlots(includeReserved) < p.slotsLimit
}

func (p *StakingPool) ReserveSlots(amount uint64) error {
	if !p.CanValidatorStake(true) {
		return errors.New("All stake slots are filled.")
	}

	p.mu.Lock()
	stakeAmount := p.stakeAmountPerSlot
	p.mu.Unlock()

	if amount%stakeAmount != 0 {
		return errors.New("Wrong amount for staking given.")
	}

	count := int(amount / stakeAmount)
	if count == 0 {
		return errors.New("Too low amount for staking.")
	}

	emptySlots := p.slotsLimit-p.filledSlots(true)
	if emptySlots < count {
		return errors.New("Can't reserve all slots with given amount.")
	}

	p.mu.Lock()
	p.slotsReserved += count
	p.mu.Unlock()

	return nil
}

func (p *StakingPool) processStakeTx(tx *types.Transaction) error {
	var validatorAmount uint64
	var electorAmount uint64
	for _, out := range tx.Outputs() {
		if len(out.Node()) != common.AddressLength {
			continue
		}

		if out.Node().Hex() == common.BlackHoleAddress {
			validatorAmount += out.Amount()
		} else {
			electorAmount += out.Amount()
		}
	}

	sender := tx.From().Hex()
	if validatorAmount > 0 {
		err := p.registerValidatorStake(sender, validatorAmount)
		if err != nil {
			log.Errorf("Error proccessing stake transaction: %s", err)
			return err
		}
	}

	if electorAmount > 0 {
		err := p.registerValidatorStake(sender, electorAmount)
		if err != nil {
			log.Errorf("Error proccessing stake transaction: %s", err)
			return err
		}
	}

	return nil
}

func (p *StakingPool) processUnstakeTx(tx *types.Transaction) error {
	// count tx stake amount
	var amount uint64
	for _, in := range tx.Inputs() {
		amount += in.Amount()
	}

	// find amount to unstake
	isValidatorUnstake := false
	var validator string
	for _, out := range tx.Outputs() {
		if len(out.Node()) == common.AddressLength {
			if out.Node().Hex() == common.BlackHoleAddress {
				isValidatorUnstake = true
			} else {
				validator = out.Node().Hex()
			}

			amount -= out.Amount()
		}
	}

	if !isValidatorUnstake && utf8.RuneCountInString(validator) != 42 {
		return errors.New("Incorrect unstake tx")
	}

	var err error
	if isValidatorUnstake {
		err = p.cancelValidatorStake(tx.From().Hex(), amount)
	} else {
		err = p.cancelElectorStake(tx.From().Hex(), validator, amount)
	}

	if err != nil {
		log.Errorf("Error unstaking slots: %s.", err)
		return err
	}

	return nil
}

func (p *StakingPool) FinalizeStaking(batch []*types.Transaction) error {
	p.mu.Lock()
	p.slotsReserved = 0
	p.mu.Unlock()

	for _, tx := range batch {
		var err error

		switch tx.Type() {
		case common.StakeTxType:
			err = p.processStakeTx(tx)
		case common.UnstakeTxType:
			err = p.processUnstakeTx(tx)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (p *StakingPool) GetRewardOutputs() []*prototype.TxOutput {
	p.mu.Lock()

	if p.slotsFilled == 0 {
		p.mu.Unlock()
		return nil
	}

	// divide rewards among all validator slots
	rewardPerSlot := p.rewardPerBlock / uint64(p.slotsFilled)
	rewards := map[string]uint64{}
	for validator, stakeData := range p.validators {
		fullValidatorReward := rewardPerSlot * uint64(stakeData.SlotsFilled)

		validatorPercent := params.RaidoConfig().ValidatorRewardPercent
		if len(stakeData.Electors) > 0 {
			validatorPercent = params.RaidoConfig().ChosenValidatorRewardPercent
		}

		// todo rework rewards mechanism

		// validator takes more percent of reward if he has electors
		validatorReward := fullValidatorReward * validatorPercent / 100
		rewards[validator] += validatorReward

		// each elector will receive a reward in proportion to the bet on the validator
		electorsReward := fullValidatorReward - validatorReward
		electorsStake := stakeData.CumulativeStake - stakeData.SelfStake
		for elector, stakeAmount := range stakeData.Electors {
			rewards[elector] += electorsStake / stakeAmount * electorsReward
		}
	}
	p.mu.Unlock()

	outs := make([]*prototype.TxOutput, 0, len(rewards))
	for addr, amount := range rewards {
		addr := common.HexToAddress(addr)
		outs = append(outs, types.NewOutput(addr.Bytes(), amount, nil))
	}

	return outs
}

func (p *StakingPool) GetRewardPerSlot(slots uint64) uint64 {
	return p.rewardPerBlock / slots
}

func (p *StakingPool) HasElector(validator, elector string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	v, exists := p.validators[validator]
	if !exists {
		return false
	}

	_, exists = v.Electors[elector]
	return exists
}

func NewPool(blockchain consensus.BlockchainReader, slotsLimit int, reward uint64, stakeAmount uint64) consensus.StakePool {
	return &StakingPool{
		validators: map[string]*ValidatorStakeData{},
		blockchain: blockchain,
		slotsLimit: slotsLimit,
		stakeAmountPerSlot: stakeAmount,
		rewardPerBlock: reward,
	}
}