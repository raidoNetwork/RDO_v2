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

		var err error
		txRes, err = s.getTransactionByLink(tx, blockIndex)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return txRes, nil
}

// updateBlockAccountState updates transaction map for all block data.
func (s *Store) updateBlockAccountState(tx *bolt.Tx, block *prototype.Block) error {
	//blockKey := genBlockKey(block.Num, block.Hash)

	bkt := tx.Bucket(addressBucket)

	for _, transaction := range block.Transactions {
		if transaction.Type == common.FeeTxType || transaction.Type == common.RewardTxType {
			continue
		}

		addr := transaction.Inputs[0].Address
		key := genAddrKey(addr)

		buf := bkt.Get(key)
		if buf == nil {
			buf = make([]byte, 0, 8)
			buf = ssz.MarshalUint64(buf, transaction.Num)
		} else {
			nonce := ssz.UnmarshallUint64(buf)

			if nonce != transaction.Num && nonce+1 != transaction.Num {
				return errors.Errorf("Wrong nonce given: %d. Expected: %d or %d", transaction.Num, nonce, nonce+1)
			}

			buf = ssz.MarshalUint64(nil, nonce)
		}

		err := bkt.Put(key, buf)
		if err != nil {
			return err
		}
	}

	return nil
}

// getTransactionByLink find transaction with given link of block hash and tx index.
func (s *Store) getTransactionByLink(tx *bolt.Tx, lnk []byte) (*prototype.Transaction, error) {
	size := len(lnk)
	blockKey := lnk[:size-4]
	index32 := ssz.UnmarshallUint32(lnk[size-4:])

	blockBkt := tx.Bucket(blocksBucket)
	blockBuf := blockBkt.Get(blockKey)

	if blockBuf == nil {
		return nil, errors.Errorf("Undefined block with key %s", blockKey)
	}

	block, err := unmarshalBlock(blockBuf)
	if err != nil {
		return nil, errors.Errorf("Block unmarshal error: %s", err)
	}

	index := int(index32)
	if index < 0 {
		return nil, errors.Errorf("Wrong transaciton index %d", index)
	}

	if index > len(block.Transactions) {
		return nil, errors.Errorf("Wrong transaction index %d with block transaction count: %d", index, len(block.Transactions))
	}

	return block.Transactions[index], nil
}

// GetTransactionsCount return nonce of given address
func (s *Store) GetTransactionsCount(addr []byte) (uint64, error) {
	key := genAddrKey(addr)
	var nonce uint64 = 0

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(addressBucket)
		data := bkt.Get(key)

		if data == nil {
			return nil
		}

		nonce = ssz.UnmarshallUint64(data)
		return nil
	})

	return nonce, err
}

func genTxHashKey(hash []byte) []byte {
	return append(transactionPrefix, hash...)
}

func genAddrKey(addr []byte) []byte {
	return append(addressPrefix, addr...)
}
