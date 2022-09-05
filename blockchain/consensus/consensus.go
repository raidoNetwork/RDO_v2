package consensus

import (
	"github.com/raidoNetwork/RDO_v2/shared/common"
)

type PoA interface {
	IsLeader(address common.Address) bool

	IsValidator(address common.Address) bool
}
