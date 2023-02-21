package staking

import (
	"math/big"
	"math/rand"
	"sync"
	"unicode/utf8"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "StakePool")

var ErrNoStake = errors.New("no stake from address")

type ValidatorStakeData struct {
	SlotsFilled     int
	CumulativeStake uint64
	Electors        map[string]uint64 // Staker => StakedAmount
	SelfStake       uint64
}

type StakingPool struct {
	validators         map[string]*ValidatorStakeData
	electors           map[string]map[string]struct{}
	unstakedSnapshot   map[string]map[string]uint64
	cumulativeStake    uint64
	stakeAmountPerSlot uint64
	slotsLimit         int
	slotsFilled        int
	slotsReserved      int
	blockchain         consensus.BlockchainReader

	mu sync.Mutex
}

func (p *StakingPool) Init() error {
	deposits, err := p.blockchain.FindStakeDeposits()
	if err != nil {
		return err
	}

	for _, uo := range deposits {
		if len(uo.Node) != common.AddressLength {
			continue
		}

		if uo.Node.Hex() == common.BlackHoleAddress {
			err = p.registerValidatorStake(uo.To.Hex(), uo.Amount)
		} else {
			err = p.registerElectorStake(uo.From.Hex(), uo.Node.Hex(), uo.Amount)
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

func (p *StakingPool) cancelValidatorStake(validator string, amount uint64) error {
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
		p.MarkFullUnstake(validator)
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
		log.Debugf("Not found validator %s with elector %s; unstaking", validator, elector)
		p.SystemUnstakeElector(validator, elector, amount)
		return nil
	}

	validatorData := p.validators[validator]
	validatorData.CumulativeStake += amount
	validatorData.Electors[elector] += amount

	if _, exists := p.electors[elector]; !exists {
		p.electors[elector] = map[string]struct{}{}
	}

	p.electors[elector][validator] = struct{}{}
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

	unstake, unstakeNeeded := p.unstakedSnapshot[validator][elector]
	if unstakeNeeded {
		if unstake > amount {
			p.unstakedSnapshot[validator][elector] -= amount
		} else {
			delete(p.unstakedSnapshot[validator], elector)
		}
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

	emptySlots := p.slotsLimit - p.filledSlots(true)
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
	electorStaking := map[string]uint64{}
	for _, out := range tx.Outputs() {
		if len(out.Node()) != common.AddressLength {
			continue
		}

		node := out.Node().Hex()
		if node == common.BlackHoleAddress {
			validatorAmount += out.Amount()
		} else {
			electorStaking[node] += out.Amount()
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

	if len(electorStaking) > 0 {
		for validator, amount := range electorStaking {
			err := p.registerElectorStake(sender, validator, amount)
			if err != nil {
				log.Errorf("Error proccessing stake transaction: %s", err)
				return err
			}
		}
	}

	return nil
}

func (p *StakingPool) processUnstakeTx(tx *types.Transaction) (err error) {
	stakeNode := ""
	var amount uint64 // count tx stake amount
	for _, in := range tx.Inputs() {
		node := in.Node().Hex()
		if stakeNode == "" {
			stakeNode = node
		} else if stakeNode != node {
			return errors.Errorf("Unstake from different nodes is not allowed. Given %s, %s", stakeNode, node)
		}

		amount += in.Amount()
	}

	// find amount to unstake
	isValidatorUnstake := stakeNode == common.BlackHoleAddress
	for _, out := range tx.Outputs() {
		if len(out.Node()) == common.AddressLength {
			amount -= out.Amount()
		}
	}

	if !isValidatorUnstake && utf8.RuneCountInString(stakeNode) != 42 {
		return errors.New("Incorrect unstake tx")
	}

	if isValidatorUnstake {
		err = p.cancelValidatorStake(tx.From().Hex(), amount)
	} else {
		err = p.cancelElectorStake(tx.From().Hex(), stakeNode, amount)
	}

	if err != nil {
		log.Errorf("Error unstaking slots: %s.", err)
		return err
	}

	return nil
}

func (p *StakingPool) processSystemUnstakeTx(tx *types.Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Calculate the inputs amounts for each address
	// record the corresponding node address
	if len(tx.Outputs()) != 1 {
		return errors.New("The length of systemUnstakeTx's outputs is not 1")
	}

	if len(tx.Inputs()) < 1 {
		return errors.New("The length of systemUnstakeTx's inputs is less than 1")
	}

	elector := tx.Outputs()[0].Address().Hex()
	amount := tx.Outputs()[0].Amount()
	validator := tx.Inputs()[0].Node().Hex()

	staked, exists := p.unstakedSnapshot[validator][elector]
	if !exists {
		log.Debugf("Not found validator %s with elector %s", validator, elector)
		return errors.New("Not found elector in the snapshot")
	}

	// Updating snapshop
	if staked == amount {
		// Remove from unstakeSnapshop if needed
		delete(p.unstakedSnapshot[validator], elector)
	} else {
		p.unstakedSnapshot[validator][elector] -= amount
	}

	p.cumulativeStake -= amount
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
		case common.ValidatorsUnstakeTxType:
			err = p.processSystemUnstakeTx(tx)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (p *StakingPool) GetRewardMap(proposer string) map[string]uint64 {
	p.mu.Lock()

	if p.slotsFilled == 0 {
		p.mu.Unlock()
		return map[string]uint64{}
	}

	rewards := map[string]uint64{}
	if _, exists := p.validators[proposer]; exists {
		blockReward := params.RaidoConfig().ProposerReward
		stakeData := p.validators[proposer]

		validatorReward := blockReward * params.RaidoConfig().ChosenValidatorRewardPercent / 100
		electorsReward := blockReward - validatorReward
		electorsStake := stakeData.CumulativeStake - stakeData.SelfStake
		for elector, stakeAmount := range stakeData.Electors {
			// We must use big ints; otherwise, we will face an overflow
			bigStake := big.NewInt(int64(stakeAmount))
			bigElectorsReward := big.NewInt(int64(electorsReward))
			bigElectorsStake := big.NewInt(int64(electorsStake))

			res1 := big.Int{}
			res2 := big.Int{}
			res1.Mul(bigStake, bigElectorsReward)
			res2.Div(&res1, bigElectorsStake)

			reward := uint64(res2.Int64())
			if reward == 0 {
				continue
			}

			rewards[elector] += reward
		}
		rewards[proposer] += validatorReward
	}

	p.mu.Unlock()
	return rewards
}

func (p *StakingPool) GetRewardOutputs(proposer string) []*prototype.TxOutput {
	rewards := p.GetRewardMap(proposer)

	outs := make([]*prototype.TxOutput, 0, len(rewards))
	for addr, amount := range rewards {
		addr := common.HexToAddress(addr)
		outs = append(outs, types.NewOutput(addr.Bytes(), amount, nil))
	}

	return outs
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

func (p *StakingPool) HasValidator(validator string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, exists := p.validators[validator]
	return exists
}

func (p *StakingPool) ValidatorStakeMap() map[string]uint64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	stakeMap := map[string]uint64{}
	for v, data := range p.validators {
		stakeMap[v] = data.CumulativeStake
	}
	return stakeMap
}

func NewPool(blockchain consensus.BlockchainReader, slotsLimit int, stakeAmount uint64) consensus.StakePool {
	return &StakingPool{
		validators:         map[string]*ValidatorStakeData{},
		electors:           map[string]map[string]struct{}{},
		unstakedSnapshot:   make(map[string]map[string]uint64),
		blockchain:         blockchain,
		slotsLimit:         slotsLimit,
		stakeAmountPerSlot: stakeAmount,
	}
}

func (s *StakingPool) GetElectorsOfValidator(validator string) (map[string]uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stakeData, exists := s.validators[validator]
	if !exists {
		return nil, errors.Errorf("Such validator does not exists")
	}
	return stakeData.Electors, nil
}

func (s *StakingPool) NumberStakers(validator string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if data, exists := s.validators[validator]; !exists {
		return 0
	} else {
		return len(data.Electors)
	}
}

func (s *StakingPool) ListValidators() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var res []string
	for key := range s.validators {
		res = append(res, key)
	}
	return res
}

func (s *StakingPool) DetermineProposer(seed int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	weights := make(map[string]uint64, 0)

	slotUnit := params.RaidoConfig().StakeSlotUnit
	coefficient := params.RaidoConfig().ElectorsCoefficient

	var cumulative uint64

	for validator, stakeData := range s.validators {
		electorsStake := stakeData.CumulativeStake - stakeData.SelfStake
		w := stakeData.SelfStake/slotUnit*100 + electorsStake/slotUnit*uint64(100*coefficient)

		weights[validator] = w
		cumulative += w
	}

	// If cumulative is zero, select the first validator in the validators list
	cfg := params.ConsensusConfig()
	proposers := cfg.Proposers
	if cumulative == 0 {
		proposer := proposers[0]
		return proposer
	}

	rand.Seed(seed)
	random := uint64(rand.Int63n(int64(cumulative))) + 1
	var sum uint64
	var leader string

	for _, validator := range proposers {
		weight, exists := weights[validator]
		if !exists {
			continue
		}
		if weight+sum >= random {
			leader = validator
			break
		}

		sum += weight
	}

	return leader
}

func (s *StakingPool) GetFullyUnstaked() map[string]map[string]struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make(map[string]map[string]struct{})
	for key1, map1 := range s.unstakedSnapshot {
		result[key1] = make(map[string]struct{})
		for key2 := range map1 {
			result[key1][key2] = struct{}{}
		}
	}
	return result
}

// MarkFullUnstake marks the validator as one performing full unstake
// Need to acquire lock outside.
func (s *StakingPool) MarkFullUnstake(validator string) {
	for elector := range s.validators[validator].Electors {
		if _, exists := s.unstakedSnapshot[validator][elector]; !exists {
			if _, exists := s.unstakedSnapshot[validator]; !exists {
				s.unstakedSnapshot[validator] = make(map[string]uint64)
			}

			s.unstakedSnapshot[validator][elector] = s.validators[validator].Electors[elector]
		}
	}
}

// SystemUnstakeElector unstakes elector if there is no validator found.
// Need to acquire lock outside.
func (s *StakingPool) SystemUnstakeElector(validator, elector string, amount uint64) {
	if _, exists := s.unstakedSnapshot[validator]; !exists {
		s.unstakedSnapshot[validator] = make(map[string]uint64)
	}
	if _, exists := s.unstakedSnapshot[validator][elector]; !exists {
		s.unstakedSnapshot[validator][elector] = amount
	} else {
		s.unstakedSnapshot[validator][elector] += amount
	}
}
