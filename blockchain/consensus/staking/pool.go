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
)

var log = logrus.WithField("prefix", "stake pool")

func NewPool(blockchain consensus.BlockchainReader, slotsLimit int, reward uint64, stakeAmount uint64) consensus.StakePool {
	p := &Pool{
		slots:         make([]string, 0, slotsLimit),
		blockReward:   reward,
		stakeAmount:   stakeAmount,
		slotsLimit: slotsLimit,
		blockchain: blockchain,
	}

	return p
}

type Pool struct {
	blockReward   uint64   // fixed reward per block
	slotsLimit    int      // slots limit
	slots         []string // address list
	reservedSlots int
	mu            sync.RWMutex
	stakeAmount   uint64

	blockchain consensus.BlockchainReader
}

func (p *Pool) LoadData() error {
	deposits, err := p.blockchain.FindStakeDeposits()
	if err != nil {
		return err
	}

	for _, uo := range deposits {
		err = p.registerStake(uo.To.Hex(), uo.Amount)
		if err != nil {
			return err
		}
	}

	log.Warnf("Stake deposits loaded. Count: %d", len(p.slots))
	stakeSlotsFilled.Set(float64(len(p.slots)))

	return nil
}

// CanStake shows if there are free slots for staking
func (p *Pool) CanStake(includeReserved bool) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.filledSlots(includeReserved) < p.slotsLimit
}

func (p *Pool) filledSlots(includeReserved bool) int {
	slotsCount := len(p.slots)
	if includeReserved {
		slotsCount += p.reservedSlots
	}
	return slotsCount
}

// ReserveSlots add address to reserved slots
func (p *Pool) ReserveSlots(amount uint64) error {
	if !p.CanStake(true) {
		return errors.New("All stake slots are filled.")
	}

	p.mu.Lock()
	stakeAmount := p.stakeAmount
	p.mu.Unlock()

	if amount%stakeAmount != 0 {
		return errors.New("Wrong amount for staking given.")
	}

	count := amount / stakeAmount

	if count == 0 {
		return errors.New("Too low amount for staking.")
	}

	emptySlots := uint64(p.slotsLimit-p.filledSlots(true))
	if emptySlots < count {
		return errors.New("Can't reserve all slots with given amount.")
	}

	p.mu.Lock()
	p.reservedSlots += int(count)
	p.mu.Unlock()

	return nil
}

// registerStake close validator slots
func (p *Pool) registerStake(address string, amount uint64) error {
	empty := p.getEmptySlots()
	if empty == 0 {
		return errors.New("Validator slots limit is reached.")
	}

	if empty < 0 {
		return errors.New("Validator slots inconsistent.")
	}


	count := amount / p.stakeAmount
	if uint64(empty) < count {
		return errors.Errorf("Can't feel all slots. Empty: %d. Given: %d.", empty, count)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var i uint64
	for i = 0; i < count; i++ {
		p.slots = append(p.slots, address)
	}
	stakeSlotsFilled.Set(float64(len(p.slots)))

	return nil
}

// unregisterStake open validator slots
func (p *Pool) unregisterStake(address string, amount uint64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	count := amount / p.stakeAmount
	if count == 0 {
		return errors.Errorf("Bad unstake amount given: %d.", amount)
	}

	var i uint64
	for i = 0;i < count;i++ {
		for k, slotAddress := range p.slots {
			if slotAddress == address {
				p.slots = append(p.slots[:k], p.slots[k+1:]...)
				break
			}
		}
	}

	stakeSlotsFilled.Set(float64(len(p.slots)))

	return nil
}

func (p *Pool) getEmptySlots() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.slotsLimit - len(p.slots)
}

func (p *Pool) GetRewardPerSlot(slots uint64) uint64 {
	return p.blockReward / slots
}

func (p *Pool) updateStakeSlots(batch []*types.Transaction) error {
	for _, tx := range batch {
		var err error

		switch tx.Type() {
		case common.StakeTxType:
			err = p.addStakeSlots(tx)
		case common.UnstakeTxType:
			err = p.removeStakeSlots(tx)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Pool) addStakeSlots(tx *types.Transaction) error {
	var amount uint64
	for _, out := range tx.Outputs() {
		if out.Node().Hex() == common.BlackHoleAddress {
			amount += out.Amount()
		}
	}

	err := p.registerStake(tx.From().Hex(), amount)
	if err != nil {
		log.Errorf("Error proccessing stake transaction: %s", err)
		return err
	}

	return nil
}

func (p *Pool) removeStakeSlots(tx *types.Transaction) error {
	// count tx stake amount
	var amount uint64
	for _, in := range tx.Inputs() {
		amount += in.Amount()
	}

	// find amount to unstake
	for _, out := range tx.Outputs() {
		if out.Node().Hex() == common.BlackHoleAddress {
			amount -= out.Amount()
		}
	}

	err := p.unregisterStake(tx.From().Hex(), amount)
	if err != nil {
		log.Errorf("Error unstaking slots: %s.", err)
		return err
	}

	return nil
}

func (p *Pool) FinalizeStaking(batch []*types.Transaction) error {
	p.mu.Lock()
	p.reservedSlots = 0
	p.mu.Unlock()

	err := p.updateStakeSlots(batch)
	if err != nil {
		return err
	}

	return nil
}

func (p *Pool) GetRewardOutputs() []*prototype.TxOutput {
	p.mu.Lock()

	size := len(p.slots)
	if size == 0 {
		p.mu.Unlock()
		return nil
	}

	// divide rewards among all validator slots
	rewardPerSlot := params.RaidoConfig().RewardBase / uint64(size)
	rewards := map[string]uint64{}
	for _, addrHex := range p.slots {
		rewards[addrHex] += rewardPerSlot
	}
	p.mu.Unlock()

	outs := make([]*prototype.TxOutput, 0, len(rewards))
	for addr, amount := range rewards {
		addr := common.HexToAddress(addr)
		outs = append(outs, types.NewOutput(addr.Bytes(), amount, nil))
	}

	return outs
}
