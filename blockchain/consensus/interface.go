package consensus

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

// TxValidator validates only transactions
type TxValidator interface {
	// ValidateTransaction validate transaction and return an error if something is wrong
	ValidateTransaction(*types.Transaction) error

	// ValidateTransactionStruct validates transaction balances, signatures and hash.
	ValidateTransactionStruct(*types.Transaction) error
}

// BlockValidator checks only blocks
type BlockValidator interface {
	// ValidateBlock validate block and return an error if something is wrong
	ValidateBlock(*prototype.Block) (*types.Transaction, error)
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
	GetBlockCount() uint64

	// FindAllUTxO find all address unspent outputs
	FindAllUTxO(string) ([]*types.UTxO, error)

	// ParentHash return parent block hash for current block
	ParentHash() []byte

	// ProcessBlock update all given block inputs and outputs state in the SQL database.
	ProcessBlock(*prototype.Block) error

	// SyncData syncs data in the KV with data in the SQL.
	SyncData() error

	// CheckBalance check if system balance is correct
	CheckBalance() error
}

// TxPool provides and updates transaction queue.
type TxPool interface {
	// GetQueue returns transaction sort queue.
	GetQueue() []*types.Transaction

	// Finalize remove from pool given transactions.
	Finalize([]*types.Transaction)

	// DeleteTransaction remove given transaction from the pool
	DeleteTransaction(*types.Transaction) error

	// InsertCollapseTx insert collapse tx to the pool
	InsertCollapseTx(*types.Transaction) error
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

	// LoadData load initial pool data
	LoadData() error
}

// AttestationPool control block and transaction validation and staking
type AttestationPool interface{
	StakePool() StakePool

	TxPool() TxPool

	Validator() Validator
}
