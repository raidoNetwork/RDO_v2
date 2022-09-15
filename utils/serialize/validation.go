package serialize

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/validator/types"
)

func MarshalAttestation(att *types.Attestation) ([]byte, error) {
	if att == nil {
		return nil, errors.New("empty attestation given")
	}

	return att.MarshalSSZ()
}

func UnmarshalAttestation(buf []byte) (*types.Attestation, error) {
	att := types.Attestation{}

	err := att.UnmarshalSSZ(buf)
	if err != nil {
		log.Errorf("Unmarshal attestation error: %s", err)
		return nil, err
	}

	return &att, nil
}
