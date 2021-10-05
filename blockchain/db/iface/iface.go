package iface

import (
	"io"
	"rdo_draft/proto/prototype"
	"rdo_draft/shared/types"
)

// BlockStorage test interface for database opportunities
type BlockStorage interface {
	CountBlocks() (int, error)
	WriteBlock(*prototype.Block) error
	WriteBlockWithNumKey(*prototype.Block) error
	ReadBlock([]byte) (*prototype.Block, error)
}

// Database common database interface
type Database interface {
	io.Closer // Close() error
	DatabasePath() string
	ClearDB() error
	SaveData(key []byte, data []byte) error
	ReadAllData() ([]DataRow, error)

	BlockStorage
}

type DataRow struct {
	Timestamp int64
	Hash      []byte
}

type OutputManager interface {
	AddOutput(uo *types.UTxO) (int64, error)
	SpendOutput(uint64) error

	AddOutputWithTx(int, *types.UTxO) (int64, error)
	SpendOutputWithTx(int, uint64) error

	HealthCheck() (uint64, error)

	// FindGenesisOutput is func for searching genesis output in UTxO database.
	FindGenesisOutput(string) (*types.UTxO, error)

	// FindAllUTxO finds all unspent outputs according to user address.
	FindAllUTxO(addr string) ([]*types.UTxO, error)

	CreateTx() (int, error)
	RollbackTx(int) error
	CommitTx(int) error
}

type OutputDatabase interface {
	io.Closer

	OutputManager
}
