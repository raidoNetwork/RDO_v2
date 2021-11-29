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

// BalanceReader checks all address unspent outputs
type BalanceReader interface {
	FindAllUTxO(string) ([]*types.UTxO, error)
}

// BlockSpecifying get need info from blockchain for verifying block
type BlockSpecifying interface {
	// GenTxRoot returns root hash of Merklee tree of transactions
	// or return error if hash was not found
	GenTxRoot([]*prototype.Transaction) ([]byte, error)

	// GetBlockByHash return block with given hash from blockchain
	// if block not found return nil
	GetBlockByHash([]byte) (*prototype.Block, error)

	// GetBlockReward return reward for block with given number
	GetBlockReward() uint64

	// GetBlockCount return block count in the blockchain
	GetBlockCount() uint64
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
