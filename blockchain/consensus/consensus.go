package consensus

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/common"
)

const COMMITTEE_SIZE = 2
const SUCCESS_PERCENT = 60

type PoA interface {
	IsLeader(address common.Address) bool

	IsValidator(address common.Address) bool
}

func IsEnoughVotes(approvers, slashers int) error {
	votedCount := approvers + slashers
	votedPercent := float64(votedCount) / COMMITTEE_SIZE * 100
	if votedPercent < SUCCESS_PERCENT {
		return errors.New("Too small validators voted per block")
	}

	approvedPercent := float64(approvers) / COMMITTEE_SIZE * 100
	if approvedPercent < SUCCESS_PERCENT {
		return errors.New("Not enough approvers per block")
	}

	return nil
}