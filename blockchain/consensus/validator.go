package consensus

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

// TxValidator validates only transactions
type TxValidator interface {
	// ValidateTransaction validate transaction and return an error if something is wrong
	ValidateTransaction(*prototype.Transaction) error
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
type BalanceReader interface{
	FindAllUTxO(string) ([]*types.UTxO, error)
}

// BlockSpecifying get need info from blockchain for verifying block
type BlockSpecifying interface{
	// GenTxRoot returns root hash of Merklee tree of transactions
	// or return error if hash was not found
	GenTxRoot([]*prototype.Transaction) ([]byte, error)

	// GetBlockByNum return block with given number from blockchain
	// if block not found return nil
	GetBlockByNum(uint64) (*prototype.Block, error)

	// GetRewardForBlock return reward for block with given number
	GetRewardForBlock(uint64) uint64
}
