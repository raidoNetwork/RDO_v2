package types

import (
	"crypto/ecdsa"
	"github.com/raidoNetwork/RDO_v2/shared/common"
)

type Proposer interface {
	Key() *ecdsa.PrivateKey

	Addr() common.Address
}
