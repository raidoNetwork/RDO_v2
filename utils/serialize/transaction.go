package serialize

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

func MarshalTx(tx *prototype.Transaction) ([]byte, error) {
	if tx == nil {
		return nil, errors.New("empty tx given")
	}

	return tx.MarshalSSZ()
}

func UnmarshalTx(buf []byte) (*prototype.Transaction, error) {
	tx := prototype.Transaction{}

	err := tx.UnmarshalSSZ(buf)
	if err != nil {
		log.Errorf("Unmarshal tx error: %s", err)
		return nil, err
	}

	return &tx, nil
}
