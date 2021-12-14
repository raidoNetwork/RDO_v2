package consensus

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

// TxValidator validates only transactions
type TxValidator interface {
	// ValidateTransaction validate transaction and return an error if something is wrong
	ValidateTransaction(*prototype.Transaction) error

	// ValidateTransactionStruct validates transaction balances, signatures and hash.
	ValidateTransactionStruct(*prototype.Transaction) error
}

// BlockValidator checks only blocks
type BlockValidator interface {
	// ValidateBlock validate block and return an error if something is wrong
	ValidateBlock(*prototype.Block) error
}

// Validator checks if block or transaction is correct according to the engine rules
type Validator interface {
	BlockValidator
	TxValidator
}

// OutputsReader checks all address unspent outputs
type OutputsReader interface {
	// FindAllUTxO find all address unspent outputs
	FindAllUTxO(string) ([]*types.UTxO, error)

	// FindStakeDeposits find all block stake deposits
	FindStakeDeposits() ([]*types.UTxO, error)

	// FindStakeDepositsOfAddress returns all address stake outputs.
	FindStakeDepositsOfAddress(string) ([]*types.UTxO, error)
}

// BlockSpecifying get need info from blockchain for verifying block
type BlockSpecifying interface {
	// GenTxRoot returns root hash of Merklee tree of transactions
	// or return error if hash was not found
	GenTxRoot([]*prototype.Transaction) ([]byte, error)

	// GetBlockByHash return block with given hash from blockchain
	// if block not found return nil
	GetBlockByHash([]byte) (*prototype.Block, error)

	// GetBlockCount return block count in the blockchain
	GetBlockCount() uint64

	// GetTransactionsCount returns address nonce
	GetTransactionsCount([]byte) (uint64, error)
}

type StakeValidator interface {
	// RegisterStake add stake balance with data in transaction
	RegisterStake([]byte) error

	// UnregisterStake unregister stake slots.
	UnregisterStake([]byte) error

	// CreateRewardTx creates transaction with reward for all stakers.
	CreateRewardTx(uint64) (*prototype.Transaction, error)

	// CanStake shows stake slots is filled or not.
	CanStake() bool

	// ReserveSlot mark validator slot as filled until block will be forged.
	ReserveSlot() error

	// FlushReserved flush all reserved slots
	FlushReserved()

	// GetRewardAmount return amount for reward with given size of filled slots.
	GetRewardAmount(int) uint64
}

// BlockMiner interface for any struct that can create and save block to the database
type BlockMiner interface {
	// GenerateBlock creates block struct from given transaction batch.
	GenerateBlock([]*prototype.Transaction) (*prototype.Block, error)

	// SaveBlock store block to the blockchain.
	SaveBlock(*prototype.Block) error

	// GetBlockCount return block count in the blockchain
	GetBlockCount() uint64
}

// OutputUpdater updates block outputs in the SQL and sync KV with SQL if it is need.
type OutputUpdater interface {
	// ProcessBlock update all given block inputs and outputs state in the SQL database.
	ProcessBlock(*prototype.Block) error

	// SyncData syncs data in the KV with data in the SQL.
	SyncData() error
}

// TransactionQueue provides and updates transaction queue.
type TransactionQueue interface {
	// GetTxQueue returns transaction sort queue.
	GetTxQueue() []*types.TransactionData

	// DeleteTransaction removes given transaction from pool.
	DeleteTransaction(transaction *prototype.Transaction) error

	// ReserveTransactions remove given transactions from pool
	// and mark it as reserved.
	ReserveTransactions([]*prototype.Transaction) error

	// RollbackReserved returns reserved transactions to the pool.
	RollbackReserved()

	// FlushReserved removes all reserved transactions from pool.
	FlushReserved(bool)
}