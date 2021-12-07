package validator

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"sync"
)

var log = logrus.WithField("prefix", "Attestation")

var ErrNoStakers = errors.New("No stake deposit is registered.")

func NewValidator(outm consensus.OutputsReader, slotsLimit int, reward uint64) (consensus.StakeValidator, error) {
	vg := &ValidatorGerm{
		slots:       make([]string, 0, slotsLimit),
		mu:          sync.RWMutex{},
		blockReward: reward,
		slotsLimit:  slotsLimit,
		outm:        outm,
	}

	// Load stake deposits data
	err := vg.LoadSlots()
	if err != nil {
		return nil, err
	}

	return vg, nil
}

type ValidatorGerm struct {
	blockReward uint64   // fixed reward per block
	slotsLimit  int    // slots limit
	slots       []string // address list
	mu          sync.RWMutex

	outm consensus.OutputsReader
}

func (vg *ValidatorGerm) LoadSlots() error {
	deposits, err := vg.outm.FindStakeDeposits()
	if err != nil {
		return err
	}

	for _, uo := range deposits {
		err = vg.RegisterStake(uo.To.Bytes())
		if err != nil {
			log.Error("Inconsistent stake deposits.")
			return err
		}
	}

	log.Warnf("Stake deposits successfully loaded. Count: %d", len(vg.slots))

	return nil
}

// CanStake shows if there are free slots for staking
func (vg *ValidatorGerm) CanStake() bool {
	emptySlots := vg.getEmptySlots()

	log.Warnf("Empty Stake slots count: %d", emptySlots)

	return emptySlots > 0
}

// RegisterStake close validator slots
func (vg *ValidatorGerm) RegisterStake(addr []byte) error {
	if vg.getEmptySlots() == 0 {
		return errors.New("Validator slots limit is reached.")
	}

	address := common.BytesToAddress(addr)

	vg.mu.Lock()
	vg.slots = append(vg.slots, address.Hex())
	vg.mu.Unlock()

	return nil
}

// UnregisterStake open validator slots
func (vg *ValidatorGerm) UnregisterStake(addr []byte) error {
	address := common.BytesToAddress(addr).Hex()

	vg.mu.Lock()
	defer vg.mu.Unlock()

	notFound := true
	for i, a := range vg.slots {
		if a == address {
			vg.slots = append(vg.slots[:i], vg.slots[i+1:]...)
			notFound = false
			break
		}
	}

	if notFound {
		return errors.Errorf("Undefined staker %s.", address)
	}

	return nil
}

// CreateRewardTx generates special transaction with reward to all stakers.
func (vg *ValidatorGerm) CreateRewardTx(blockNum uint64) (*prototype.Transaction, error) {
	outs := vg.createRewardOutputs()

	if len(outs) == 0 {
		return nil, ErrNoStakers
	}

	opts := types.TxOptions{
		Outputs: outs,
		Type:    common.RewardTxType,
		Fee:     0,
		Num:     blockNum,
	}

	ntx, err := types.NewTx(opts, nil)
	if err != nil {
		return nil, err
	}

	return ntx, nil
}

// createRewardOutputs
func (vg *ValidatorGerm) createRewardOutputs() []*prototype.TxOutput {
	vg.mu.RLock()
	size := len(vg.slots)
	slots := vg.slots
	reward := vg.blockReward
	vg.mu.RUnlock()

	data := make([]*prototype.TxOutput, 0, size)
	if size == 0 {
		return data
	}

	// divide reward among all validator slots
	reward /= uint64(size)

	for _, addrHex := range slots {
		addr := common.HexToAddress(addrHex)
		data = append(data, types.NewOutput(addr.Bytes(), reward, nil))
	}

	return data
}

func (vg *ValidatorGerm) getEmptySlots() int {
	vg.mu.RLock()
	defer vg.mu.RUnlock()

	return vg.slotsLimit - len(vg.slots)
}

func (vg *ValidatorGerm) GetRewardAmount(size int) uint64 {
	vg.mu.RLock()
	defer vg.mu.RUnlock()

	if size == 0 {
		return 0
	}

	return vg.blockReward / uint64(size)
}