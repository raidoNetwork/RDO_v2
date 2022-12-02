package poa

import (
	"sort"
	"sync"

	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
)

type Backend struct {
	validators []common.Address

	mu sync.Mutex
}

func New() *Backend {
	cfg := params.ConsensusConfig()
	validators := make([]common.Address, 0, len(cfg.Proposers))

	sort.Strings(cfg.Proposers)

	for _, proposer := range cfg.Proposers {
		validators = append(validators, common.HexToAddress(proposer))
	}

	return &Backend{
		validators: validators,
	}
}

func (b *Backend) IsLeader(validator common.Address) bool {
	return validator.Hex() == b.Leader().Hex()
}

func (b *Backend) leaderIndex() int {
	validatorsCount := uint64(len(b.validators))
	slot := slot.Ticker().SlotSinceGenesis()
	ind := slot % validatorsCount
	return int(ind)
}

func (b *Backend) Leader() common.Address {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.validators[b.leaderIndex()]
}

func (b *Backend) IsValidator(user common.Address) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	userAddress := user.Hex()
	for _, v := range b.validators {
		if v.Hex() == userAddress {
			return true
		}
	}

	return false
}
