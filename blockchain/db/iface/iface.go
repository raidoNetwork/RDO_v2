package iface

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"io"
)

// BlockStorage interface for work with blocks
type BlockStorage interface {
	WriteBlock(*prototype.Block) error
	CountBlocks() (int, error)

	HeadAccessStorage
	BlockReader
	TransactionMapper
}

// BlockReader interface to access blocks
type BlockReader interface {
	GetBlock(num uint64, hash []byte) (*prototype.Block, error)
	GetBlockByNum(num uint64) (*prototype.Block, error)
	GetBlockByHash(hash []byte) (*prototype.Block, error)
}

type HeadAccessStorage interface {
	// SaveHeadBlockNum saves head block number to the database
	SaveHeadBlockNum(uint64) error

	// GetHeadBlockNum returns head block number.
	// If head block num was not found in the database return error.
	GetHeadBlockNum() (uint64, error)

	// SaveGenesis saves Genesis block to the storage.
	SaveGenesis(*prototype.Block) error

	// GetGenesis returns Genesis block
	GetGenesis() (*prototype.Block, error)
}

type TransactionMapper interface {
	// GetTransactionByHash find tx with given hash in the database.
	GetTransactionByHash([]byte) (*prototype.Transaction, error)
}

// Database common database interface
type Database interface {
	io.Closer // Close() error
	DatabasePath() string
	ClearDB() error

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

	StakeStorage
}

type StakeStorage interface {
	// FindStakeDeposits returns list of all actual stake deposits.
	FindStakeDeposits() ([]*types.UTxO, error)

	// FindStakeDepositsOfAddress returns list of given address actual stake deposits.
	FindStakeDepositsOfAddress(string) ([]*types.UTxO, error)
}

type OutputDatabase interface {
	io.Closer

	OutputStorage
}

// SQLConfig create config for any SQL database.
type SQLConfig struct {
	ShowFullStat bool
	ConfigPath   string
	DataDir      string
}
