package kv

import (
	"encoding/hex"
	ssz "github.com/ferranbt/fastssz"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	bolt "go.etcd.io/bbolt"
	"strings"
	"time"
)

var ErrNoHead = errors.New("Header block was not found")

// WriteBlock useful func for testings database.
func (s *Store) WriteBlock(block *prototype.Block) error {
	start := time.Now()

	data, err := serialize.MarshalBlock(block)
	if err != nil {
		return errors.Wrap(err, "Marshaling block error")
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
		log.Debugf("WriteBlock: Insert block in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.updateBlockAccountState(tx, block); err != nil {
			log.Error("Error updating block accounts state.")
			return err
		}

		end = time.Since(start)
		log.Debugf("WriteBlock: Update accounts state in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.saveBlockNum(tx, block.Num, block.Hash); err != nil {
			log.Error("Error saving block num into db.")
			return err
		}

		end = time.Since(start)
		log.Debugf("WriteBlock: Save block num in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.saveBlockSlot(tx, block.Slot, block.Num); err != nil {
			log.Error("Error saving block slot into db.")
			return err
		}

		end = time.Since(start)
		log.Debugf("WriteBlock: Save block slot in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.saveBlockHash(tx, block.Num, block.Hash); err != nil {
			log.Error("Error saving block hash into db.")
			return err
		}

		end = time.Since(start)
		log.Debugf("WriteBlock: Save block hash in %s", common.StatFmt(end))

		start = time.Now()
		if err := s.saveTransactions(tx, block); err != nil {
			log.Error("Error saving block transactions map.")
			return err
		}

		end = time.Since(start)
		log.Debugf("WriteBlock: Save transaction in hash map in %s", common.StatFmt(end))

		return nil
	})
}

// GetBlock returns block from database by key if found otherwise returns error.
func (s *Store) GetBlock(num uint64, hash []byte) (*prototype.Block, error) {
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
		log.Debugf("Read block from db in %s.", common.StatFmt(time.Since(start)))

		var err error
		blk, err = serialize.UnmarshalBlock(enc)
		if err != nil {
			return err
		}

		// save stat for reading db
		log.Debugf("Read and unmarshal end in %s.", common.StatFmt(time.Since(start)))

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
		return nil, errors.Errorf("Not found hash mapped to given number %d.", num)
	}

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

func (s *Store) GetBlockBySlot(slot uint64) (*prototype.Block, error) {
	var num uint64
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksSlotBucket)
		val := bkt.Get(genSlotKey(slot))

		if len(val) != 8 {
			return errors.Errorf("Not found block num connected to given slot %d.", slot)
		}

		num = ssz.UnmarshallUint64(val)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.GetBlockByNum(num)
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

// saveBlockSlot saves block slot with num key
func (s *Store) saveBlockSlot(tx *bolt.Tx, slot, num uint64) error {
	key := genSlotKey(slot)
	buf := make([]byte, 0)
	buf = ssz.MarshalUint64(buf, num)
	return tx.Bucket(blocksSlotBucket).Put(key, buf)
}

// saveBlockNum store block num with hash key
func (s *Store) saveBlockNum(tx *bolt.Tx, num uint64, hash []byte) error {
	key := genNumKey(hash)
	buf := make([]byte, 0)
	buf = ssz.MarshalUint64(buf, num)
	return tx.Bucket(blocksNumBucket).Put(key, buf)
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

// GetBlockNum returns block num by block hash
func (s *Store) GetBlockNum(hash []byte) (uint64, error) {
	var res uint64 = 0

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksNumBucket)
		val := bkt.Get(genNumKey(hash))

		if len(val) != 8 {
			return errors.Errorf("Not found block num connected to given hash %s.", hex.EncodeToString(hash))
		}

		res = ssz.UnmarshallUint64(val)
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
		blk, err = serialize.UnmarshalBlock(enc)
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

	data, err := serialize.MarshalBlock(block)
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

// UpdateAmountStats updates statistics for net amount check.
func (s *Store) UpdateAmountStats(reward, fee uint64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(addressBucket)
		raw := bkt.Get(statsKey)

		if raw == nil {
			return bkt.Put(statsKey, marshalStats(reward, fee))
		} else {
			var feeTotal uint64
			rewardTotal, feeTotal := unmarshalStats(raw)

			rewardTotal += reward
			feeTotal += fee

			return bkt.Put(statsKey, marshalStats(rewardTotal, feeTotal))
		}
	})
}

func (s *Store) GetAmountStats() (uint64, uint64) {
	var reward, fee uint64

	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(addressBucket)
		raw := bkt.Get(statsKey)

		if raw == nil {
			reward = 0
			fee = 0

			return nil
		}

		reward, fee = unmarshalStats(raw)

		return nil
	})

	if err != nil {
		log.Error("Error selecting amount stats:", err)
	}

	return reward, fee
}

// todo use sync pool instead
var statsBuf = make([]byte, 0, 16)

func marshalStats(counter uint64, fee uint64) []byte {
	raw := statsBuf
	raw = ssz.MarshalUint64(raw[:8], counter)
	raw = ssz.MarshalUint64(raw[8:], fee)

	defer func() {
		statsBuf = statsBuf[:0]
	}()

	return raw
}

func unmarshalStats(raw []byte) (uint64, uint64) {
	counter := ssz.UnmarshallUint64(raw[:8])
	fee := ssz.UnmarshallUint64(raw[8:])
	return counter, fee
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

func genSlotKey(slot uint64) []byte {
	key := blockSlotPrefix
	key = ssz.MarshalUint64(key, slot)
	return key
}