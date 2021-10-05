package db

import (
	"context"
	"rdo_draft/blockchain/db/kv"
	"rdo_draft/blockchain/db/utxo"
)

// NewDB initializes a new DB.
func NewDB(ctx context.Context, dirPath string, config *kv.Config) (Database, error) {
	return kv.NewKVStore(ctx, dirPath, config)
}

// NewUTxODB initializes a new UTxO DB.
func NewUTxODB(ctx context.Context, dirPath string, config *utxo.Config) (OutputDatabase, error) {
	return utxo.NewUTxOStore(ctx, dirPath, config)
}
