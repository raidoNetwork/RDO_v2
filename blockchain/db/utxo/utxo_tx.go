package utxo

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

const (
	simpleTxType = iota
	lockTxType   // lock database closing until tx with this type commit or rollback
)

// AddOutput add new tx output to the database
func (s *Store) AddOutput(txID int, uo *types.UTxO) (rows int64, err error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()
	if !exists {
		return 0, errors.Errorf("Undefined transaction #%d", txID)
	}

	query := `INSERT INTO "` + utxoTable + `" (tx_type, hash, tx_index, address_from, address_to, amount, timestamp, spent, blockId, address_node) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := tx.Exec(query, uo.TxType, uo.Hash.Hex(), uo.Index, uo.From.Hex(), uo.To.Hex(), uo.Amount, uo.Timestamp, common.UnspentTxO, uo.BlockNum, uo.Node.Hex())
	if err != nil {
		return
	}

	rows, err = res.RowsAffected()
	if err != nil {
		return
	}

	if rows != 1 && s.cfg.ShowFullStat {
		log.Debugf("Execute query: INSERT INTO %s (tx_type, hash, tx_index, address_from, address_to, amount, timestamp, spent, blockId, address_node) VALUES (%d, %s, %d, %s, %s, %d, %d, %d, %d, %s)", utxoTable, uo.TxType, uo.Hash, uo.Index, uo.From, uo.To, uo.Amount, uo.Timestamp, common.UnspentTxO, uo.BlockNum, uo.Node)
	}

	return
}

// SpendOutput delete output in the database
func (s *Store) SpendOutput(txID int, hash string, index uint32) (int64, error) {
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

	// rows can be equal to 0 when sync databases
	if rows > 1 && s.cfg.ShowFullStat {
		log.Debugf("Execute query: DELETE FROM %s WHERE hash = %s AND tx_index = %d", utxoTable, hash, index)
	}

	return rows, nil
}

// VerifyOutput check if there is a given output in the database.
func (s *Store) VerifyOutput(txID int, uo *types.UTxO) (int, error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()

	if !exists {
		return 0, errors.Errorf("OutputDB.VerifyOutput: Undefined transaction #%d", txID)
	}

	query := `SELECT COUNT(id) FROM ` + utxoTable + ` WHERE blockId = ? AND hash = ? AND tx_index = ? AND address_to = ? AND amount = ? AND tx_type = ? AND timestamp = ? AND address_node = ?`

	rows, err := tx.Query(query, uo.BlockNum, uo.Hash.Hex(), uo.Index, uo.To.Hex(), uo.Amount, uo.TxType, uo.Timestamp, uo.Node.Hex())
	if err != nil {
		return 0, err
	}

	var affectedRows int
	for rows.Next() {
		err = rows.Scan(&affectedRows)
		if err != nil {
			return 0, err
		}
	}

	return affectedRows, nil
}

// DeleteOutputs delete all outputs with given block number.
func (s *Store) DeleteOutputs(txID int, blockNum uint64) error {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()

	if !exists {
		return errors.Errorf("OutputDB.VerifyOutput: Undefined transaction #%d", txID)
	}

	query := `DELETE FROM ` + utxoTable + ` WHERE blockId = ?`

	_, err := tx.Exec(query, blockNum)
	if err != nil {
		return err
	}

	return nil
}

// CreateTx create new database transaction and return it's ID.
func (s *Store) CreateTx() (int, error) {
	s.lock.RLock()
	canCreate := s.canCreateTx
	s.lock.RUnlock()

	if !canCreate {
		return 0, errors.New("Database is closing.")
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return 0, err
	}

	s.lock.Lock()
	s.tx[s.txID] = tx
	s.txStatus[s.txID] = simpleTxType
	id := s.txID
	s.txID++
	s.lock.Unlock()

	if s.cfg.ShowFullStat {
		log.Debugf("OutputDB.CreateTx: new database tx id #%d.", id)
	}

	return id, nil
}

// RollbackTx rollback database transaction.
func (s *Store) RollbackTx(txID int) error {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()
	if !exists {
		return errors.Errorf("OutputDB.RollbackTx: Undefined transaction #%d", txID)
	}

	defer func() {
		s.lock.Lock()
		delete(s.tx, txID)
		delete(s.txStatus, txID)
		s.lock.Unlock()
	}()

	err := tx.Rollback()
	if err != nil {
		return err
	}

	return nil
}

// CommitTx commit database transaction.
func (s *Store) CommitTx(txID int) (err error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()
	if !exists {
		return errors.Errorf("OutputDB.CommitTx: Undefined transaction #%d", txID)
	}

	defer func() {
		s.lock.Lock()
		delete(s.tx, txID)
		delete(s.txStatus, txID)
		s.lock.Unlock()
	}()

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

// finishWriting prepare database for closing
func (s *Store) finishWriting() {
	s.lock.Lock()
	s.canCreateTx = false
	s.lock.Unlock()
}
