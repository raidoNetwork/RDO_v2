package kv

import (
	"context"
	"encoding/binary"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
	"os"
	"path"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/iface"
	"github.com/raidoNetwork/RDO_v2/shared/fileutil"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"time"
)

const (
	// RdoNodeDbDirName is the name of the directory containing the rdo node database.
	RdoNodeDbDirName = "raido"
	// DatabaseFileName is the name of the rdo node database.
	DatabaseFileName = "raido.db"

	boltAllocSize = 8 * 1024 * 1024
	// The size of hash length in bytes
	hashLength = 32
)

var log = logrus.WithField("prefix", "database")

// BlockCacheSize specifies 1000 slots worth of blocks cached, which
// would be approximately 2MB
var BlockCacheSize = int64(1 << 21)

// Config for the bolt db kv store.
type Config struct {
	InitialMMapSize int
}

// Store defines an implementation of the database.
type Store struct {
	db           *bolt.DB
	databasePath string
	ctx          context.Context
}

// NewKVStore initializes a new boltDB key-value store at the directory
// path specified, creates the kv-buckets based on the schema, and stores
// an open connection db object as a property of the Store struct.
func NewKVStore(ctx context.Context, dirPath string, config *Config) (*Store, error) {
	hasDir, err := fileutil.HasDir(dirPath)
	if err != nil {
		return nil, err
	}
	if !hasDir {
		if err := fileutil.MkdirAll(dirPath); err != nil {
			return nil, err
		}
	}
	datafile := KVStoreDatafilePath(dirPath)
	boltDB, err := bolt.Open(
		datafile,
		params.RaidoIoConfig().ReadWritePermissions,
		&bolt.Options{
			Timeout:         1 * time.Second,
			InitialMmapSize: config.InitialMMapSize,
		},
	)
	if err != nil {
		if errors.Is(err, bolt.ErrTimeout) {
			return nil, errors.New("cannot obtain database lock, database may be in use by another process")
		}
		return nil, err
	}
	boltDB.AllocSize = boltAllocSize

	kv := &Store{
		db:           boltDB,
		databasePath: dirPath,
		ctx:          ctx,
	}

	if err := kv.db.Update(func(tx *bolt.Tx) error {
		return createBuckets(
			tx,
			blocksBucket,
		)
	}); err != nil {
		return nil, err
	}

	return kv, err
}

// KVStoreDatafilePath is the canonical construction of a full
// database file path from the directory path, so that code outside
// this package can find the full path in a consistent way.
func KVStoreDatafilePath(dirPath string) string {
	return path.Join(dirPath, DatabaseFileName)
}

func createBuckets(tx *bolt.Tx, buckets ...[]byte) error {
	for _, bucket := range buckets {
		if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
			return err
		}
	}
	return nil
}

// ClearDB removes the previously stored database in the data directory.
func (s *Store) ClearDB() error {
	if _, err := os.Stat(s.databasePath); os.IsNotExist(err) {
		return nil
	}
	//prometheus.Unregister(createBoltCollector(s.db))
	if err := os.Remove(path.Join(s.databasePath, DatabaseFileName)); err != nil {
		return errors.Wrap(err, "could not remove database file")
	}
	return nil
}

// Close closes the underlying BoltDB database.
func (s *Store) Close() error {
	return s.db.Close()
}

// DatabasePath at which this database writes files.
func (s *Store) DatabasePath() string {
	return s.databasePath
}

func (s *Store) SaveData(key []byte, data []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)

		exists := bkt.Get(key)
		if exists != nil {
			return errors.New("error adding data to db. Key is already exists")
		}

		if err := bkt.Put(key, data); err != nil {
			return err
		}

		return nil
	})
}

func (s *Store) ReadAllData() ([]iface.DataRow, error) {
	res := make([]iface.DataRow, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		bkt := tx.Bucket(blocksBucket)
		c := bkt.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			key := binary.BigEndian.Uint64(k)
			data := iface.DataRow{
				Timestamp: int64(key),
				Hash:      v,
			}

			res = append(res, data)
		}

		return nil
	})

	return res, err
}
