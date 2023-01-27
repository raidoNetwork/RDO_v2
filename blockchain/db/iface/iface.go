package iface

import (
	"io"

	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

// ChainStorage interface for work with blocks
type ChainStorage interface {
	WriteBlock(*prototype.Block) error
	CountBlocks() (int, error)

	UpdateAmountStats(uint64, uint64) error
	GetAmountStats() (uint64, uint64)

	HeadAccessStorage
	BlockReader
	TransactionReader
}

// BlockReader interface to access blocks
type BlockReader interface {
	GetBlock(num uint64, hash []byte) (*prototype.Block, error)
	GetBlockByNum(num uint64) (*prototype.Block, error)
	GetBlockByHash(hash []byte) (*prototype.Block, error)
	GetBlockBySlot(slot uint64) (*prototype.Block, error)
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

type TransactionReader interface {
	// GetTransactionByHash find tx with given hash in the database.
	GetTransactionByHash([]byte) (*prototype.Transaction, error)

	// GetTransactionsCount return nonce of given address
	GetTransactionsCount([]byte) (uint64, error)
}

// Database common database interface
type Database interface {
	io.Closer // Close() error
	DatabasePath() string
	ClearDB() error

	ChainStorage
}

type OutputStorage interface {
	// AddOutputIfNotExists - add unspent output to the database in database transaction.
	AddOutputIfNotExists(int, *types.UTxO) error

	// AddOutputBatch add outputs batch data to the database
	AddOutputBatch(int, string) (int64, error)

	// FindAllUTxO finds all unspent outputs according to user address.
	FindAllUTxO(string) ([]*types.UTxO, error)

	// CreateTx creates database transaction and returns it's ID.
	CreateTx(bool) (int, error)

	// CommitTx - commits transaction with given ID if it exists.
	CommitTx(int) error

	// RollbackTx - rollbacks all changes made by tx.
	RollbackTx(int) error

	SpendOutput(txID int, hash string, index uint32) (int64, error)

	// FindLastBlockNum find maximum block id in the SQL
	FindLastBlockNum() (uint64, error)

	// DeleteOutputs remove all outputs with given block num.
	DeleteOutputs(int, uint64) error

	GetTotalAmount() (uint64, error)

	ClearDatabase() error

	StakeStorage

	// Ping the mysql database to keep the connection alive
	Ping() error
}

type StakeStorage interface {
	// FindStakeDeposits returns list of all actual stake deposits.
	FindStakeDeposits() ([]*types.UTxO, error)

	// FindStakeDepositsOfAddress returns list of given address actual stake deposits.
	FindStakeDepositsOfAddress(string, string) ([]*types.UTxO, error)

	// FindValidatorStakeDeposits returns list of all validators stake deposits
	FindValidatorStakeDeposits() ([]*types.UTxO, error)
}

type OutputDatabase interface {
	io.Closer

	OutputStorage
}

// SQLConfig create config for any SQL database.
type SQLConfig struct {
	ConfigPath string
	DataDir    string
}
