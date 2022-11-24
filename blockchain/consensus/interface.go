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
	ValidateBlock(*prototype.Block, TxJournal, bool) ([]*types.Transaction, error)

	// ValidateGenesis compare given Genesis with local
	ValidateGenesis(*prototype.Block) error
}

// Validator checks if block or transaction is correct according to the engine rules
type Validator interface {
	BlockValidator
	TxValidator
}

type GenesisReader interface {
	// GetGenesis returns Genesis block
	GetGenesis() *prototype.Block
}

type BlockchainReader interface {
	// FindAllUTxO find all address unspent outputs
	FindAllUTxO(string) ([]*types.UTxO, error)

	// FindStakeDeposits find all block stake deposits
	FindStakeDeposits() ([]*types.UTxO, error)

	// FindValidatorStakeDeposits find all stake validator deposits
	FindValidatorStakeDeposits() ([]*types.UTxO, error)

	// FindStakeDepositsOfAddress returns all address stake outputs.
	FindStakeDepositsOfAddress(string, string) ([]*types.UTxO, error)

	// GetBlockByHash return block with given hash from blockchain
	// if block not found return nil
	GetBlockByHash([]byte) (*prototype.Block, error)

	// GetBlockCount return block count in the blockchain
	GetBlockCount() uint64

	// GetTransactionsCount returns address nonce
	GetTransactionsCount([]byte) (uint64, error)

	GenesisReader
}

// BlockFinalizer interface for any struct that can create and save block to the database
type BlockFinalizer interface {
	// FinalizeBlock store block to the blockchain.
	FinalizeBlock(*prototype.Block, []*types.Transaction) error

	// GetBlockCount return block count in the blockchain
	GetBlockCount() uint64

	// FindAllUTxO find all address unspent outputs
	FindAllUTxO(string) ([]*types.UTxO, error)

	// ParentHash return parent block hash for current block
	ParentHash() []byte

	// SyncData syncs data in the KV with data in the SQL.
	SyncData() error

	// CheckBalance check if system balance is correct
	CheckBalance() error

	GenesisReader
}

// TxPool provides and updates transaction queue.
type TxPool interface {
	// GetQueue returns transaction sort queue.
	GetQueue() []*types.Transaction

	// Finalize remove from pool given transactions.
	Finalize([]*types.Transaction, bool)

	// DeleteTransaction remove given transaction from the pool
	DeleteTransaction(*types.Transaction) error

	// InsertCollapseTx insert collapse tx to the pool
	InsertCollapseTx(*types.Transaction) error

	TxJournal
}

type TxJournal interface {
	IsKnown(*types.Transaction) bool
}

// StakePool regulates stake slots condition.
type StakePool interface {
	// CanValidatorStake shows stake slots is filled or not.
	CanValidatorStake(bool) bool

	// ReserveSlots mark validator slots as filled until block will be forged.
	ReserveSlots(uint64) error

	// GetRewardOutputs return array of reward outputs
	GetRewardOutputs(string) []*prototype.TxOutput

	GetRewardMap(string) map[string]uint64

	// Init load initial pool data
	Init() error

	// FinalizeStaking complete all staking pool updates
	FinalizeStaking([]*types.Transaction) error

	// GetRewardPerSlot return reward per slot
	GetRewardPerSlot(uint64) uint64

	HasValidator(validator string) bool
}

// AttestationPool control block and transaction validation and staking
type AttestationPool interface {
	StakePool() StakePool

	TxPool() TxPool

	Validator() Validator
}
