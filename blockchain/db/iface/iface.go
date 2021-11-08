package iface

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"io"
)

// BlockStorage interface for work with blocks
type BlockStorage interface {
	WriteBlock(*prototype.Block) error
	WriteBlockWithNumKey(*prototype.Block) error
	CountBlocks() (int, error)
	ReadBlock([]byte) (*prototype.Block, error)
	ReadBlockWithNumkey(num uint64) (*prototype.Block, error)
}

// Database common database interface
type Database interface {
	io.Closer // Close() error
	DatabasePath() string
	ClearDB() error
	SaveData(key []byte, data []byte) error
	ReadAllData() ([]DataRow, error)

	BlockStorage
}

type DataRow struct {
	Timestamp int64
	Hash      []byte
}

type OutputStorage interface {
	// AddOutput - add unspent output to the database in database transaction.
	AddOutput(int, *types.UTxO) (int64, error)

	// FindAllUTxO finds all unspent outputs according to user address.
	FindAllUTxO(string) ([]*types.UTxO, error)

	// VerifyOutput check that database is in the database
	VerifyOutput(int, *types.UTxO) (int, error)

	// CreateTx creates database transaction and returns it's ID.
	CreateTx() (int, error)

	// CommitTx - commits transaction with given ID if it exists.
	CommitTx(int) error

	// RollbackTx - rollbacks all changes maded by tx.
	RollbackTx(int) error

	SpendOutput(txID int, hash string, index uint32) (int64, error)

	// FindLastBlockNum find maximum block id in the SQL
	FindLastBlockNum() (uint64, error)

	// DeleteOutputs remove all outputs with given block num.
	DeleteOutputs(int, uint64) error

	GetTotalAmount() (uint64, error)
}

type OutputDatabase interface {
	io.Closer

	OutputStorage
}
