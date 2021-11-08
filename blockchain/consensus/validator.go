package consensus

import "github.com/raidoNetwork/RDO_v2/proto/prototype"

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
