package consensus

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	log "github.com/sirupsen/logrus"
)

const SUCCESS_PERCENT = 60

type PoA interface {
	IsLeader(address common.Address) bool

	IsValidator(address common.Address) bool
}

func IsEnoughVotes(approvers, slashers int) error {
	commiteeSize := float64(params.ConsensusConfig().CommitteeSize)
	votedCount := approvers + slashers
	votedPercent := float64(votedCount) / commiteeSize * 100

	log.Debugf(
		"Approvers: %d Slashers: %d VotedPercent: %f",
		approvers,
		slashers,
		votedPercent,
	)

	if votedPercent < SUCCESS_PERCENT {
		return errors.New("Too small validators voted per block")
	}

	approvedPercent := float64(approvers) / commiteeSize * 100
	if approvedPercent < SUCCESS_PERCENT {
		return errors.New("Not enough approvers per block")
	}

	return nil
}