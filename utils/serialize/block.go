package serialize

import (
	"github.com/golang/snappy"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "serialize")

func UnmarshalBlock(enc []byte) (*prototype.Block, error) {
	rawBlock := &prototype.Block{}

	var err error
	enc, err = snappy.Decode(nil, enc)
	if err != nil {
		return rawBlock, err
	}

	err = rawBlock.UnmarshalSSZ(enc)
	if err != nil {
		log.Errorf("Unmarshal block error: %s", err)
		return rawBlock, err
	}

	return rawBlock, nil
}

func MarshalBlock(blk *prototype.Block) ([]byte, error) {
	if blk == nil {
		return nil, errors.New("empty block given")
	}

	obj, err := blk.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	return snappy.Encode(nil, obj), nil
}
