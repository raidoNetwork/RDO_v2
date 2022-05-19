package staking

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/sirupsen/logrus"
	"sync"
)

var log = logrus.WithField("prefix", "stake pool")

func NewPool(outm consensus.BlockchainReader, slotsLimit int, reward uint64, stakeAmount uint64) consensus.StakePool {
	p := &Pool{
		slots:         make([]string, 0, slotsLimit),
		reservedSlots: make([]int, 0),
		blockReward:   reward,
		stakeAmount:   stakeAmount,

		slotsLimit: slotsLimit,
		outm:       outm,
	}

	return p
}

type Pool struct {
	blockReward   uint64   // fixed reward per block
	slotsLimit    int      // slots limit
	slots         []string // address list
	reservedSlots []int
	mu            sync.RWMutex
	stakeAmount   uint64

	outm consensus.BlockchainReader
}

func (p *Pool) LoadData() error {
	deposits, err := p.outm.FindStakeDeposits()
	if err != nil {
		return err
	}

	for _, uo := range deposits {
		err = p.registerStake(uo.To, uo.Amount)
		if err != nil {
			log.Error("Inconsistent stake deposits.")
			return err
		}
	}

	log.Warnf("Stake deposits successfully loaded. Count: %d", len(p.slots))

	return nil
}

// CanStake shows if there are free slots for staking
func (p *Pool) CanStake() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.filledSlots() < p.slotsLimit
}

func (p *Pool) filledSlots() int {
	return len(p.slots) + len(p.reservedSlots)
}

// ReserveSlots add address to reserved slots
func (p *Pool) ReserveSlots(amount uint64) error {
	if !p.CanStake() {
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

	if uint64(p.slotsLimit-p.filledSlots()) < count {
		return errors.New("Can't reserve all slots with given amount.")
	}

	p.mu.Lock()

	var i uint64
	for i = 0; i < count; i++ {
		p.reservedSlots = append(p.reservedSlots, 1)
	}

	p.mu.Unlock()

	return nil
}

// FlushReservedSlots flush all reserved validator slots
func (p *Pool) FlushReservedSlots() {
	p.mu.Lock()
	p.reservedSlots = make([]int, 0)
	p.mu.Unlock()
}

// registerStake close validator slots
func (p *Pool) registerStake(addr []byte, amount uint64) error {
	empty := p.getEmptySlots()
	if empty == 0 {
		return errors.New("Validator slots limit is reached.")
	}

	if empty < 0 {
		return errors.New("Validator slots inconsistent.")
	}

	p.mu.Lock()
	stakeAmount := p.stakeAmount
	p.mu.Unlock()

	address := common.BytesToAddress(addr).Hex()
	count := amount / stakeAmount

	if uint64(empty) < count {
		return errors.Errorf("Can't feel all slots. Empty: %d. Given: %d.", empty, count)
	}

	p.mu.Lock()
	var i uint64
	for i = 0; i < count; i++ {
		p.slots = append(p.slots, address)
	}
	p.mu.Unlock()

	return nil
}

// unregisterStake open validator slots
func (p *Pool) unregisterStake(addr []byte, amount uint64) error {
	address := common.BytesToAddress(addr).Hex()

	p.mu.Lock()
	defer p.mu.Unlock()

	stakeAmount := p.stakeAmount
	count := amount / stakeAmount

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

	return nil
}

func (p *Pool) getEmptySlots() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.slotsLimit - len(p.slots)
}

func (p *Pool) GetStakeSlots() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.slots
}

func (p *Pool) GetRewardPerSlot(slots uint64) uint64 {
	return p.blockReward / slots
}

func (p *Pool) UpdateStakeSlots(block *prototype.Block) error {
	var sender []byte
	var err error

	for _, tx := range block.Transactions {
		if tx.Type == common.StakeTxType || tx.Type == common.UnstakeTxType {
			sender = tx.Inputs[0].Address
			var amount uint64

			if tx.Type == common.StakeTxType {
				for _, out := range tx.Outputs {
					if common.BytesToAddress(out.Node).Hex() == common.BlackHoleAddress {
						amount += out.Amount
					}
				}

				err = p.registerStake(sender, amount)
				if err != nil {
					log.Errorf("Error proccessing stake transaction: %s", err)
					return err
				}
			} else {
				// count tx stake amount
				for _, in := range tx.Inputs {
					amount += in.Amount
				}

				// find amount to unstake
				for _, out := range tx.Outputs {
					if common.BytesToAddress(out.Node).Hex() == common.BlackHoleAddress {
						amount -= out.Amount
					}
				}

				err = p.unregisterStake(sender, amount)
				if err != nil {
					log.Errorf("Error unstaking slots: %s.", err)
					return err
				}
			}
		}
	}

	return nil
}



