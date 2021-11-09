package kv

import (
	"encoding/hex"
	ssz "github.com/ferranbt/fastssz"
	"github.com/golang/snappy"
	"github.com/pkg/errors"
	types "github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	bolt "go.etcd.io/bbolt"
	"time"
)

// WriteBlock useful func for testings database.
// It writes blocks with key Keccak256(block num + block suffix)
func (s *Store) WriteBlock(block *types.Block) error {
	start := time.Now()

	data, err := marshalBlock(block)
	if err != nil {
		log.Error("Block marshal error")
		return err
	}

	end := time.Since(start)
	log.Infof("Marshal block in %s", common.StatFmt(end))

	// generate key
	key := genKey(int(block.Num))

	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		if exists := bkt.Get(key); exists != nil {
			return errors.Errorf("block #%s already exists", hex.EncodeToString(key))
		}

		if err := bkt.Put(key, data); err != nil {
			log.Error("Error saving into db")
			return err
		}

		return nil
	})
}

// ReadBlock returns block from database by key if found otherwise returns error.
func (s *Store) ReadBlock(num uint64) (*types.Block, error) {
	// generate key
	key := genKey(int(num))

	var blk *types.Block
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

// CountBlocks iterates the whole database and count number of rows in it.
func (s *Store) CountBlocks() (int, error) {
	num := 0
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		c := bkt.Stats()

		num = c.KeyN

		return nil
	})

	return num, err
}

func unmarshalBlock(enc []byte) (*types.Block, error) {
	rawBlock := &types.Block{}

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

func marshalBlock(blk *types.Block) ([]byte, error) {
	obj, err := blk.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	return snappy.Encode(nil, obj), nil
}

func genKey(num int) []byte {
	nbyte := make([]byte, 0)
	nbyte = ssz.MarshalUint64(nbyte, uint64(num))
	nbyte = append(nbyte, BlockSuffix...)
	key := crypto.Keccak256(nbyte)

	return key
}
