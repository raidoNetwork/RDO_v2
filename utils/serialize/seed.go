package serialize

import (
	"github.com/golang/snappy"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

func UnmarshalSeed(enc []byte) (*prototype.Seed, error) {
	rawSeed := &prototype.Seed{}

	var err error
	enc, err = snappy.Decode(nil, enc)
	if err != nil {
		return rawSeed, err
	}

	err = rawSeed.UnmarshalSSZ(enc)
	if err != nil {
		log.Errorf("Unmarshal block error: %s", err)
		return rawSeed, err
	}

	return rawSeed, nil
}

func MarshalSeed(seed *prototype.Seed) ([]byte, error) {
	if seed == nil {
		return nil, errors.New("empty block given")
	}

	obj, err := seed.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	return snappy.Encode(nil, obj), nil
}
