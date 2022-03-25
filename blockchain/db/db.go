package db

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/kv"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/utxo"
)

// NewDB initializes a new DB.
func NewDB(ctx context.Context, dirPath string) (Database, error) {
	return kv.NewKVStore(ctx, dirPath)
}

// NewUTxODB initializes a new UTxO DB.
func NewUTxODB(ctx context.Context, config *SQLConfig) (OutputDatabase, error) {
	return utxo.NewStore(ctx, config)
}
