package db

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/kv"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/utxo"
)

// NewDB initializes a new DB.
func NewDB(ctx context.Context, dirPath string, config *kv.Config) (Database, error) {
	return kv.NewKVStore(ctx, dirPath, config)
}

// NewUTxODB initializes a new UTxO DB.
func NewUTxODB(ctx context.Context, dirPath string, config *utxo.Config) (OutputDatabase, error) {
	return utxo.NewUTxOStore(ctx, dirPath, config)
}
