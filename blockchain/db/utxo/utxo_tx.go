package utxo

import (
	"context"
	"encoding/hex"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

// AddOutputWithTx add new tx output to the database
func (s *Store) AddOutputWithTx(txID int, uo *types.UTxO) (rows int64, err error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()
	if !exists {
		return 0, errors.Errorf("Undefined transaction #%d", txID)
	}

	query := `INSERT INTO "` + utxoTable + `" (tx_type, hash, tx_index, address_from, address_to, amount, timestamp, spent, blockId) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	hash := hex.EncodeToString(uo.Hash)
	from := hex.EncodeToString(uo.From)
	to := hex.EncodeToString(uo.To)

	res, err := tx.Exec(query, uo.TxType, hash, uo.Index, from, to, uo.Amount, uo.Timestamp, common.UnspentTxO, uo.BlockNum)
	if err != nil {
		return
	}

	rows, err = res.RowsAffected()
	if err != nil {
		return
	}

	if rows != 1 {
		log.Debugf("Execute query: INSERT INTO %s (tx_type, hash, tx_index, address_from, address_to, amount, timestamp, spent, blockId) VALUES (%d, %s, %d, %s, %s, %d, %d, %d, %d)", utxoTable, uo.TxType, hash, uo.Index, from, to, uo.Amount, uo.Timestamp, common.UnspentTxO, uo.BlockNum)
	}

	return
}

// AddNodeOutputWithTx add new tx output to the database
func (s *Store) AddNodeOutputWithTx(txID int, uo *types.UTxO) (rows int64, err error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()
	if !exists {
		return 0, errors.Errorf("Undefined transaction #%d", txID)
	}

	query := `INSERT INTO "` + utxoTable + `" (tx_type, hash, tx_index, address_node, amount, timestamp, spent, blockId) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	hash := hex.EncodeToString(uo.Hash)
	to := hex.EncodeToString(uo.To)

	res, err := tx.Exec(query, uo.TxType, hash, uo.Index, to, uo.Amount, uo.Timestamp, common.UnspentTxO, uo.BlockNum)
	if err != nil {
		return
	}

	rows, err = res.RowsAffected()
	if err != nil {
		return
	}

	if rows != 1 {
		log.Debugf("Execute query: INSERT INTO %s (tx_type, hash, tx_index, address_node, amount, timestamp, spent, blockId) VALUES (%d, %s, %d, %s, %d, %d, %d, %d)", utxoTable, uo.TxType, hash, uo.Index, to, uo.Amount, uo.Timestamp, common.UnspentTxO, uo.BlockNum)
	}

	return
}

func (s *Store) SpendOutputWithTx(txID int, hash string, index uint32) (int64, error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()
	if !exists {
		return 0, errors.Errorf("Undefined transaction #%d", txID)
	}

	query := `DELETE FROM ` + utxoTable + ` WHERE hash = ? AND tx_index = ?`

	res, err := tx.Exec(query, hash, index)
	if err != nil {
		return 0, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	if rows != 1 {
		log.Debugf("Execute query: DELETE FROM %s WHERE hash = %s AND tx_index = %d", utxoTable, hash, index)
	}

	return rows, nil
}

func (s *Store) SpendGenesis(txID int, address string) (rows int64, err error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()

	if !exists {
		return 0, errors.Errorf("OutputDB.SpendGenesis: Undefined transaction #%d", txID)
	}

	query := `UPDATE ` + utxoTable + ` SET spent = ? WHERE address_to = ? AND tx_type = ? ;`

	res, err := tx.Exec(query, common.SpentTxO, address, common.GenesisTxType)
	if err != nil {
		return 0, err
	}

	rows, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}

	if rows != 1 {
		log.Debugf("Execute query: UPDATE %s SET spent = %d WHERE address_to = %s AND tx_type = %d ;", utxoTable, common.SpentTxO, address, common.GenesisTxType)
	}

	return
}

// CreateTx create new database transaction and return it's ID.
func (s *Store) CreateTx() (int, error) {
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}

	s.lock.Lock()
	s.tx[s.txID] = tx
	id := s.txID
	s.txID++
	s.lock.Unlock()

	log.Debugf("OutputDB.CreateTx: new database tx id #%d.", id)

	return id, nil
}

func (s *Store) RollbackTx(txID int) error {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()
	if !exists {
		return errors.Errorf("OutputDB.RollbackTx: Undefined transaction #%d", txID)
	}

	err := tx.Rollback()
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) CommitTx(txID int) (err error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()
	if !exists {
		return errors.Errorf("OutputDB.CommitTx: Undefined transaction #%d", txID)
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}
