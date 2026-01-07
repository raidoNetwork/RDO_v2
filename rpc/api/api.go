package api

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

type ChainAPI interface {
	/* Address data */

	// GetBalance return address balance in Rei.
	GetBalance(string) (uint64, error)

	// FindAllUTxO returns unspent outputs of given address.
	FindAllUTxO(string) ([]*types.UTxO, error)

	// GetStakeDeposits returns address stake deposits.
	GetStakeDeposits(string, string) ([]*types.UTxO, error)

	// GetTransactionsCountHex returns number of transactions
	// sent by given address.
	GetTransactionsCountHex(string) (uint64, error)

	/* Blockdata */

	// GetBlockByHashHex return block with given hash.
	GetBlockByHashHex(string) (*prototype.Block, error)

	// GetBlockByNum return block with given num.
	GetBlockByNum(uint64) (*prototype.Block, error)

	// GetLatestBlock
	GetLatestBlock() (*prototype.Block, error)

	/* Transaction data */

	// GetTransaction return transaction with given hash.
	GetTransaction(string) (*prototype.Transaction, error)

	/* Miscellaneous data */

	// GetSystemBalance returns the total balance in the system (market cap)
	GetSystemBalance() uint64

	/* Status data */

	// GetSyncStatus return node sync status SQL with KV.
	GetSyncStatus() (string, error)

	// GetServiceStatus return service status or error if service is offline.
	GetServiceStatus() (string, error)

	// GetBlocksStartCount returns a number of blocks starting at some number
	GetBlocksStartCount(start int64, limit uint32) ([]*prototype.Block, error)
}

type AttestationAPI interface {
	// SendRawTx send transaction to the TxPool.
	SendRawTx(transaction *prototype.Transaction) error

	// GetFee return minimal fee needed for adding transaction to the block.
	GetFee() uint64

	// GetServiceStatus return service status or error if service is offline.
	GetServiceStatus() (string, error)

	// GetPendingTransactions returns list of pending transactions.
	GetPendingTransactions() ([]*prototype.Transaction, error)

	// StakersLimitReached returns an error if
	// the limit of stakers for a particular validator is exceeded
	StakersLimitReached(tx *types.Transaction) error

	// IsNodeValidator returns an error if
	// the node is not a validator
	IsNodeValidator(node string) error

	// ListValidators returns a list of working validators
	ListValidators() []string

	// ListStakeValidators returns a list of validators that can be staken on
	ListStakeValidators() []string
}

type GeneratorAPI interface {
	GenerateUnsafeTx([]*prototype.TxOutput, uint64, string) (*prototype.Transaction, error)

	GenerateUnsafeStakeTx(uint64, string, uint64, string) (*prototype.Transaction, error)

	GenerateUnsafeUnstakeTx(uint64, string, uint64, string) (*prototype.Transaction, error)

	GenerateTx([]*prototype.TxOutput, uint64, string) (*prototype.Transaction, error)

	GenerateStakeTx(uint64, string, uint64, string) (*prototype.Transaction, error)

	GenerateUnstakeTx(uint64, string, uint64, string) (*prototype.Transaction, error)
}
