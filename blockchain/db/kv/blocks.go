package kv

import (
	"encoding/hex"
	ssz "github.com/ferranbt/fastssz"
	"github.com/golang/snappy"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	bolt "go.etcd.io/bbolt"
	"strings"
	"time"
)

var ErrNoHead = errors.New("Header block was not found")

// WriteBlock useful func for testings database.
func (s *Store) WriteBlock(block *prototype.Block) error {
	start := time.Now()

	data, err := marshalBlock(block)
	if err != nil {
		log.Error("Block marshal error")
		return err
	}

	end := time.Since(start)
	log.Debugf("Marshal block in %s", common.StatFmt(end))

	// generate key
	key := genBlockKey(block.Num, block.Hash)

	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		if exists := bkt.Get(key); exists != nil {
			return errors.Errorf("block #%s already exists", hex.EncodeToString(key))
		}

		start = time.Now()
		if err := bkt.Put(key, data); err != nil {
			log.Error("Error saving block into db.")
			return err
		}

		end = time.Since(start)
		log.Infof("WriteBlock: Insert block in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.updateBlockAccountState(tx, block); err != nil {
			log.Error("Error updating block accounts state.")
			return err
		}

		end = time.Since(start)
		log.Infof("WriteBlock: Update accounts state in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.saveBlockNum(tx, block.Num, block.Hash); err != nil {
			log.Error("Error saving block num into db.")
			return err
		}

		end = time.Since(start)
		log.Infof("WriteBlock: Save block num in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.saveBlockHash(tx, block.Num, block.Hash); err != nil {
			log.Error("Error saving block hash into db.")
			return err
		}

		end = time.Since(start)
		log.Infof("WriteBlock: Save block hash in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.saveTransactions(tx, block); err != nil {
			log.Error("Error saving block transactions map.")
			return err
		}

		end = time.Since(start)
		log.Infof("WriteBlock: Save transaction in hash map in %s", common.StatFmt(end))

		return nil
	})
}

// GetBlock returns block from database by key if found otherwise returns error.
func (s *Store) GetBlock(num uint64, hash []byte) (*prototype.Block, error) {
	// generate key
	key := genBlockKey(num, hash)

	var blk *prototype.Block
	err := s.db.View(func(tx *bolt.Tx) error {
		start := time.Now()

		bkt := tx.Bucket(blocksBucket)
		enc := bkt.Get(key)
		if enc == nil {
			return nil
		}

		// save stat for reading db
		log.Debugf("Read row from db in %s.", common.StatFmt(time.Since(start)))

		var err error
		blk, err = unmarshalBlock(enc)
		if err != nil {
			return err
		}

		// save stat for reading db
		log.Debugf("Read row end in %s.", common.StatFmt(time.Since(start)))

		return nil
	})

	return blk, err
}

// GetBlockByNum returns block with given block number
func (s *Store) GetBlockByNum(num uint64) (*prototype.Block, error) {
	hash, err := s.GetBlockHash(num)
	if err != nil {
		return nil, err
	}

	if hash == nil {
		return nil, nil
	}

	log.Debugf("Got hash %s", hex.EncodeToString(hash))

	return s.GetBlock(num, hash)
}

// GetBlockByHash returns block with given hash
func (s *Store) GetBlockByHash(hash []byte) (*prototype.Block, error) {
	num, err := s.GetBlockNum(hash)
	if err != nil {
		if strings.Contains(err.Error(), "Not found") {
			return nil, nil
		}

		return nil, err
	}

	return s.GetBlock(num, hash)
}

// CountBlocks iterates the whole database and count number of rows in it.
func (s *Store) CountBlocks() (int, error) {
	num := 0

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		stats := bkt.Stats() // get bucket stats
		num = stats.KeyN

		return nil
	})

	return num, err
}

// SaveHeadBlockNum saves head block number to the database.
func (s *Store) SaveHeadBlockNum(num uint64) error {
	val := make([]byte, 0)
	val = ssz.MarshalUint64(val, num)

	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		return bkt.Put(lastBlockKey, val)
	})
}

// GetHeadBlockNum returns head block number
func (s *Store) GetHeadBlockNum() (uint64, error) {
	val := make([]byte, 8)
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		val = bkt.Get(lastBlockKey)

		if val == nil {
			return ErrNoHead
		}

		return nil
	})

	var res uint64 = 0
	if val != nil {
		res = ssz.UnmarshallUint64(val)
	}

	return res, err
}

// saveBlockHash saves block hash with num key
func (s *Store) saveBlockHash(tx *bolt.Tx, num uint64, hash []byte) error {
	key := genHashKey(num)
	return tx.Bucket(blocksHashBucket).Put(key, hash)
}

// GetBlockHash returns block hash by block num
func (s *Store) GetBlockHash(num uint64) ([]byte, error) {
	hash := make([]byte, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksHashBucket)
		hash = bkt.Get(genHashKey(num))
		return nil
	})

	return hash, err
}

// saveBlockNum store block num with hash key
func (s *Store) saveBlockNum(tx *bolt.Tx, num uint64, hash []byte) error {
	key := genNumKey(hash)
	buf := make([]byte, 0)
	buf = ssz.MarshalUint64(buf, num)

	return tx.Bucket(blocksNumBucket).Put(key, buf)
}

// GetBlockNum returns block num by block hash
func (s *Store) GetBlockNum(hash []byte) (uint64, error) {
	var res uint64 = 0

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksNumBucket)
		val := bkt.Get(genNumKey(hash))

		if val != nil && len(val) == 8 {
			res = ssz.UnmarshallUint64(val)
		} else {
			return errors.Errorf("Not found block num connected to given hash %s.", hex.EncodeToString(hash))
		}

		return nil
	})

	return res, err
}

// GetGenesis returns Genesis from database
func (s *Store) GetGenesis() (*prototype.Block, error) {
	var blk *prototype.Block
	err := s.db.View(func(tx *bolt.Tx) error {
		start := time.Now()

		bkt := tx.Bucket(blocksBucket)
		enc := bkt.Get(genesisBlockKey)
		if enc == nil {
			return nil
		}

		// save stat for reading db
		log.Debugf("Read Genesis from db in %s.", common.StatFmt(time.Since(start)))

		start = time.Now()

		var err error
		blk, err = unmarshalBlock(enc)
		if err != nil {
			return err
		}

		// save stat for reading db
		log.Debugf("Parse Genesis in %s.", common.StatFmt(time.Since(start)))

		return nil
	})

	return blk, err
}

// SaveGenesis save Genesis block to the database
func (s *Store) SaveGenesis(block *prototype.Block) error {
	start := time.Now()

	data, err := marshalBlock(block)
	if err != nil {
		log.Error("Genesis block marshal error")
		return err
	}

	end := time.Since(start)
	log.Debugf("Marshal block in %s", common.StatFmt(end))

	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)

		if err := bkt.Put(genesisBlockKey, data); err != nil {
			log.Error("Error saving into db")
			return err
		}

		return nil
	})
}

func unmarshalBlock(enc []byte) (*prototype.Block, error) {
	rawBlock := &prototype.Block{}

	var err error
	enc, err = snappy.Decode(nil, enc)
	if err != nil {
		return rawBlock, err
	}

	err = rawBlock.UnmarshalSSZ(enc)
	if err != nil {
		return rawBlock, err
	}

	return rawBlock, nil
}

func marshalBlock(blk *prototype.Block) ([]byte, error) {
	obj, err := blk.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	return snappy.Encode(nil, obj), nil
}

func genHashKey(num uint64) []byte {
	key := blockHashPrefix
	key = ssz.MarshalUint64(key, num)

	return key
}

func genNumKey(hash []byte) []byte {
	key := blockNumPrefix
	key = append(key, hash...)

	return key
}

func genBlockKey(num uint64, hash []byte) []byte {
	nbyte := blockPrefix
	nbyte = ssz.MarshalUint64(nbyte, num)
	nbyte = append(nbyte, hash...)

	return nbyte
}
