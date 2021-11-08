package types

import (
	ssz "github.com/ferranbt/fastssz"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
)

const (
	hashRootSize = 32
)

// TransactionHasher is helper for creating transaction hash with given options and hash func
type TransactionHasher struct {
	opts *TxOptions
}

// Hash returns tx hash with given options
// hash = Keccak256(num + fee + outputs + inputs + data)
func (th *TransactionHasher) Hash() ([]byte, error) {
	var buf []byte

	// Put Num
	buf = ssz.MarshalUint64(buf, th.opts.Num)

	// Put Fee
	buf = ssz.MarshalUint64(buf, th.opts.Fee)

	var root [32]byte
	var err error
	var size int

	// Put Outputs data
	size = len(th.opts.Outputs)
	outputsDomain := make([]byte, 0, size*hashRootSize)
	for i := 0; i < size; i++ {
		root, err = th.opts.Outputs[i].HashTreeRoot()
		if err != nil {
			return nil, err
		}

		outputsDomain = append(outputsDomain, root[:]...)
	}

	buf = append(buf, outputsDomain...)

	// Put Inputs data
	size = len(th.opts.Inputs)
	inputsDomain := make([]byte, 0, size*hashRootSize)
	for i := 0; i < size; i++ {
		root, err = th.opts.Inputs[i].HashTreeRoot()
		if err != nil {
			log.Warn("Inputs")
			return nil, err
		}

		inputsDomain = append(inputsDomain, root[:]...)
	}

	buf = append(buf, inputsDomain...)
	buf = append(buf, th.opts.Data...)

	hash := crypto.Keccak256(buf)

	return hash, nil
}
