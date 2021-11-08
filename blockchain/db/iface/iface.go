package iface

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"io"
)

// BlockStorage test interface for database opportunities
type BlockStorage interface {
	CountBlocks() (int, error)
	WriteBlock(*prototype.Block) error
	WriteBlockWithNumKey(block *prototype.Block) error
	ReadBlock([]byte) (*prototype.Block, error)
	ReadBlockWithNumkey(uint64) (*prototype.Block, error)
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

type OutputManager interface {
	// AddOutput - add unspent output to the database.
	AddOutput(*types.UTxO) (int64, error)

	// AddOutputWithTx - add unspent output to the database in database transaction.
	AddOutputWithTx(int, *types.UTxO) (int64, error)

	// AddNodeOutputWithTx - create outputs for special transactions
	AddNodeOutputWithTx(int, *types.UTxO) (int64, error)

	// HealthCheck - test function which read and print last 20 rows in order to know database is working correctly.
	HealthCheck() (uint64, error)

	// FindGenesisOutput is func for searching genesis output in UTxO database.
	FindGenesisOutput(string) (*types.UTxO, error)

	// FindAllUTxO finds all unspent outputs according to user address.
	FindAllUTxO(string) ([]*types.UTxO, error)

	// CreateTx creates database transaction and returns it's ID.
	CreateTx() (int, error)

	// CommitTx - commits transaction with given ID if it exists.
	CommitTx(int) error

	// RollbackTx - rollbacks all changes maded by tx.
	RollbackTx(int) error

	// CleanSpent - use in tests for removing all rows with spent transaction outputs.
	CleanSpent() error

	// SpendOutput - delete output with given ID.
	SpendOutput(uint64) error

	SpendOutputWithTx(txID int, hash string, index uint32) (int64, error)

	SpendGenesis(int, string) (int64, error)
}

type OutputDatabase interface {
	io.Closer

	OutputManager
}
