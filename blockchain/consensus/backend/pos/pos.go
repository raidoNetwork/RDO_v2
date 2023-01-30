package pos

import (
	"sort"
	"sync"

	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
)

type Config struct {
	AttestationPool *consensus.AttestationPool
}

type Backend struct {
	validators []common.Address
	att        *consensus.AttestationPool
	mu         sync.Mutex
}

func New(cfg Config) *Backend {
	config := params.ConsensusConfig()
	validators := make([]common.Address, 0, len(config.Proposers))

	sort.Strings(config.Proposers)

	for _, proposer := range config.Proposers {
		validators = append(validators, common.HexToAddress(proposer))
	}
	backend := Backend{
		validators: validators,
		att:        cfg.AttestationPool,
	}
	return &backend
}

func (b *Backend) IsLeader(address common.Address, seed int64) bool {
	proposer := (*b.att).StakePool().DetermineProposer(seed)
	hexAddress := address.Hex()
	return hexAddress == proposer
}

func (b *Backend) IsValidator(address common.Address) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	hex := address.Hex()
	for _, addr := range b.validators {
		if hex == addr.Hex() {
			return true
		}
	}

	return false
}
