package types

import (
	"github.com/sirupsen/logrus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"time"
)

type TxOptions struct {
	Inputs  []*prototype.TxInput
	Outputs []*prototype.TxOutput
	Fee     uint64
	Data    []byte
	Num     uint64
	Type    uint32
}

var log = logrus.WithField("prefix", "types")

// NewTx creates new transaction with given options
func NewTx(opts TxOptions) (*prototype.Transaction, error) {
	tx := new(prototype.Transaction)

	tx.Num = opts.Num
	tx.Type = opts.Type
	tx.Timestamp = uint64(time.Now().UnixNano())
	tx.Fee = opts.Fee
	tx.Data = opts.Data
	tx.Inputs = opts.Inputs
	tx.Outputs = opts.Outputs

	// TODO: create hasher interface or simple method
	hasher := TransactionHasher{
		opts: &opts,
	}

	h, err := hasher.Hash()
	if err != nil {
		log.Error("Error generating tx hash.", err)
		return nil, err
	}

	tx.Hash = h[:]
	tx.Size = uint32(tx.SizeSSZ())

	return tx, nil
}
