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

type BlockchainReader interface{
	// FindAllUTxO find all address unspent outputs
	FindAllUTxO(string) ([]*types.UTxO, error)

	// FindStakeDeposits find all block stake deposits
	FindStakeDeposits() ([]*types.UTxO, error)

	// FindStakeDepositsOfAddress returns all address stake outputs.
	FindStakeDepositsOfAddress(string) ([]*types.UTxO, error)

	// GetBlockByHash return block with given hash from blockchain
	// if block not found return nil
	GetBlockByHash([]byte) (*prototype.Block, error)

	// GetBlockCount return block count in the blockchain
	GetBlockCount() uint64

	// GetTransactionsCount returns address nonce
	GetTransactionsCount([]byte) (uint64, error)
}

// BlockForger interface for any struct that can create and save block to the database
type BlockForger interface {
	// SaveBlock store block to the blockchain.
	SaveBlock(*prototype.Block) error

	// GetGenesis returns Genesis block
	GetGenesis() *prototype.Block

	// GetBlockCount return block count in the blockchain
	GetBlockCount() uint64 // same

	// FindAllUTxO find all address unspent outputs
	FindAllUTxO(string) ([]*types.UTxO, error) // same

	// ParentHash return parent block hash for current block
	ParentHash() []byte

	// ProcessBlock update all given block inputs and outputs state in the SQL database.
	ProcessBlock(*prototype.Block) error

	// SyncData syncs data in the KV with data in the SQL.
	SyncData() error
}

// TxPool provides and updates transaction queue.
type TxPool interface {
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

// StakePool regulates stake slots condition.
type StakePool interface {
	// CanStake shows stake slots is filled or not.
	CanStake() bool

	// ReserveSlots mark validator slots as filled until block will be forged.
	ReserveSlots(uint64) error

	// FlushReservedSlots flush all reserved slots
	FlushReservedSlots()

	// GetStakeSlots returns array of stake slots
	GetStakeSlots() []string

	// GetRewardPerSlot return reward amount for each stake slot
	GetRewardPerSlot(uint64) uint64

	// UpdateStakeSlots update stake slots state according to the given block
	UpdateStakeSlots(block *prototype.Block) error
}

type AttestationPool interface{
	StakePool() StakePool

	TxPool() TxPool

	Validator() Validator
}
