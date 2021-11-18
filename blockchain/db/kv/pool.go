package kv

import (
	ssz "github.com/ferranbt/fastssz"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	bolt "go.etcd.io/bbolt"
)

// saveTransactions create map txHash -> blockHash + txIndex
func (s *Store) saveTransactions(tx *bolt.Tx, block *prototype.Block) error {
	bkt := tx.Bucket(transactionBucket)
	blockKey := genBlockKey(block.Num, block.Hash)

	for i, transaction := range block.Transactions {
		if transaction.Type != common.NormalTxType {
			continue
		}

		key := genTxHashKey(transaction.Hash)
		buf := make([]byte, 0)
		buf = ssz.MarshalUint32(buf, uint32(i))
		body := append(blockKey, buf...)
		err := bkt.Put(key, body)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetTransactionByHash find in database transaction with given hash and return it if found
func (s *Store) GetTransactionByHash(hash []byte) (*prototype.Transaction, error) {
	key := genTxHashKey(hash)
	txRes := new(prototype.Transaction)

	err := s.db.View(func(tx *bolt.Tx) error {
		txBkt := tx.Bucket(transactionBucket)
		blockIndex := txBkt.Get(key)

		if blockIndex == nil {
			return errors.New("Undefined transaction)")
		}

		size := len(blockIndex)
		blockKey := blockIndex[:size-4]
		index32 := ssz.UnmarshallUint32(blockIndex[size-4:])

		blockBkt := tx.Bucket(blocksBucket)
		blockBuf := blockBkt.Get(blockKey)

		if blockBuf == nil {
			return errors.Errorf("Undefined block with key %s", blockKey)
		}

		block, err := unmarshalBlock(blockBuf)
		if err != nil {
			return errors.Errorf("Block unmarshal error: %s", err)
		}

		index := int(index32)
		if index < 0 {
			return errors.Errorf("Wrong transaciton index %d", index)
		}

		if index > len(block.Transactions) {
			return errors.Errorf("Wrong transaction index %d with block transaction count: %d", index, len(block.Transactions))
		}

		txRes = block.Transactions[index]

		return nil
	})

	if err != nil {
		return nil, err
	}

	return txRes, nil
}

func genTxHashKey(hash []byte) []byte {
	return append(transactionPrefix, hash...)
}

