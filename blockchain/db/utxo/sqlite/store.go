package sqlite

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/iface"
	"github.com/raidoNetwork/RDO_v2/shared/fileutil"
	"path/filepath"
)

const (
	utxoPath  = "utxo"
	utxoFname = "outputs.db"
)

func NewStore(cfg *iface.SQLConfig) (*sql.DB, string, error) {
	dbPath := filepath.Join(cfg.DataDir, utxoPath)
	hasDir, err := fileutil.HasDir(dbPath)
	if err != nil {
		return nil, "", err
	}
	if !hasDir {
		if err := fileutil.MkdirAll(dbPath); err != nil {
			return nil, "", err
		}
	}

	fpath := filepath.Join(dbPath, utxoFname)
	db, err := sql.Open("sqlite3", fpath)
	if err != nil {
		return nil, "", err
	}

	return db, utxoSchema, nil
}
