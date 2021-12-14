package types

import (
	"github.com/golang/snappy"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

func MarshalTx(tx *prototype.Transaction) ([]byte, error) {
	obj, err := tx.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	return snappy.Encode(nil, obj), nil
}

func UnmarshalTx(enc []byte) (*prototype.Transaction, error) {
	rawTx := &prototype.Transaction{}

	var err error
	enc, err = snappy.Decode(nil, enc)
	if err != nil {
		return rawTx, err
	}

	err = rawTx.UnmarshalSSZ(enc)
	if err != nil {
		return rawTx, err
	}

	return rawTx, nil
}
