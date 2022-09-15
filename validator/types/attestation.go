package types

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

type AttestationType uint32

const (
	Approve AttestationType = iota
	Reject
)

var mixMap = map[AttestationType][]byte{
	Approve: {1},
	Reject:  {2},
}

type Attestation struct {
	Validator common.Address `ssz-size:"20"`
	Block     *prototype.Block
	Signature *prototype.Sign
	Type      AttestationType
}

func NewAttestation(block *prototype.Block, proposer Proposer, attestType AttestationType) (*Attestation, error) {
	if _, exists := mixMap[attestType]; !exists {
		return nil, errors.New("Unknown attestation type")
	}

	sign, err := types.GetBlockSigner().SignMixed(
		types.GetBlockHeader(block),
		mixMap[attestType],
		proposer.Key(),
	)
	if err != nil {
		return nil, err
	}

	return &Attestation{
		Validator: proposer.Addr(),
		Block: block,
		Signature: sign,
		Type: attestType,
	}, nil
}

func VerifyAttestationSign(att *Attestation) error {
	return VerifyBlockSign(
		types.NewHeader(att.Block),
		att.Type,
		att.Signature,
	)
}

func VerifyBlockSign(header *types.BlockHeader, att AttestationType, sign *prototype.Sign) error {
	if _, exists := mixMap[att]; !exists {
		return errors.New("Unknown attestation type")
	}

	return types.GetBlockSigner().VerifyMixed(
		header,
		mixMap[att],
		sign,
	)
}