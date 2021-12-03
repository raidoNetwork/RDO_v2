package miner

import "github.com/raidoNetwork/RDO_v2/proto/prototype"

// BlockMiner interface for any struct that can create and save block to the database
type BlockMiner interface {
	// GenerateBlock creates block struct from given transaction batch.
	GenerateBlock([]*prototype.Transaction) (*prototype.Block, error)

	// SaveBlock store block to the blockchain.
	SaveBlock(*prototype.Block) error

	GetBlockCount() uint64
}

// OutputUpdater updates block outputs in the SQL and sync KV with SQL if it is need.
type OutputUpdater interface {
	// ProcessBlock update all given block inputs and outputs state in the SQL database.
	ProcessBlock(*prototype.Block) error

	// SyncData syncs data in the KV with data in the SQL.
	SyncData() error
}
