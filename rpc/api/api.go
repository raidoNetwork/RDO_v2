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
	GetStakeDeposits(string) ([]*types.UTxO, error)

	// GetTransactionsCount returns number of transactions
	// sent by given address.
	GetTransactionsCount(string) (uint64, error)

	/* Block data */

	// GetBlockByHash return block with given hash.
	GetBlockByHash(string) (*prototype.Block, error)

	// GetBlockByNum return block with given num.
	GetBlockByNum(uint64) (*prototype.Block, error)

	// GetLatestBlock
	GetLatestBlock() (*prototype.Block, error)

	/* Transaction data */

	// GetTransaction return transaction with given hash.
	GetTransaction(string) (*prototype.Transaction, error)

	/* Status data */

	// GetSyncStatus return node sync status SQL with KV.
	GetSyncStatus() (string, error)

	// GetServiceStatus return service status or error if service is offline.
	GetServiceStatus() (string, error)
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
}

type GeneratorAPI interface {
	GenerateTx([]*prototype.TxOutput, uint64, string) (*prototype.Transaction, error)

	GenerateStakeTx(uint64, string, uint64) (*prototype.Transaction, error)

	GenerateUnstakeTx(uint64, string, uint64) (*prototype.Transaction, error)
}
