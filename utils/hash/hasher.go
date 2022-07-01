package hash

import (
	ssz "github.com/ferranbt/fastssz"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/sirupsen/logrus"
)

const (
	hashRootSize = 32
)

var log = logrus.WithField("prefix", "Hasher")

// BlockHash count block hash
// hash = Keccak256(num + slot + version + parentHash + txRoot + timestamp)
func BlockHash(num, slot uint64, version, parent, txroot []byte, tstamp uint64) []byte {
	res := make([]byte, 0, 8)
	res = ssz.MarshalUint64(res, num)
	res = ssz.MarshalUint64(res, slot)

	res = append(res, version...)
	res = append(res, parent...)
	res = append(res, txroot...)

	res = ssz.MarshalUint64(res, tstamp)

	h := crypto.Keccak256(res)
	res = h[:]

	return res
}

// TxHash count tx hash
// hash = Keccak256(num + fee + outputs + inputs + data + type + timestamp)
func TxHash(tx *prototype.Transaction) (common.Hash, error) {
	var buf []byte

	hash := common.Hash{}

	// Put Num
	buf = ssz.MarshalUint64(buf, tx.Num)

	// Put Fee
	buf = ssz.MarshalUint64(buf, tx.Fee)

	var root [32]byte
	var err error
	var size int

	// Put Outputs data
	size = len(tx.Outputs)
	outputsDomain := make([]byte, 0, size*hashRootSize)
	for i := 0; i < size; i++ {
		root, err = tx.Outputs[i].HashTreeRoot()
		if err != nil {
			log.Errorf("Outputs marshal error: %s", err)
			return hash, err
		}

		outputsDomain = append(outputsDomain, root[:]...)
	}

	buf = append(buf, outputsDomain...)

	// Put Inputs data
	size = len(tx.Inputs)
	inputsDomain := make([]byte, 0, size*hashRootSize)
	for i := 0; i < size; i++ {
		root, err = tx.Inputs[i].HashTreeRoot()
		if err != nil {
			log.Errorf("Inputs marshal error: %s", err)
			return hash, err
		}

		inputsDomain = append(inputsDomain, root[:]...)
	}

	buf = append(buf, inputsDomain...)
	buf = append(buf, tx.Data...)

	//put num
	buf = ssz.MarshalUint32(buf, tx.Type)

	//put timestamp
	buf = ssz.MarshalUint64(buf, tx.Timestamp)

	hash = crypto.Keccak256Hash(buf)

	return hash, nil
}