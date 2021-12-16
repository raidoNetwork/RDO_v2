package utxo

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/utxo/dbshared"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

// AddOutputIfNotExists add new tx output to the database
func (s *Store) AddOutputIfNotExists(txID int, uo *types.UTxO) (err error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()

	if !exists {
		return errors.Errorf("Undefined transaction #%d", txID)
	}

	query := `INSERT INTO ` + dbshared.UtxoTable + ` (tx_type, hash, tx_index, address_from, address_to, amount, timestamp, blockId, address_node) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE tx_type = ?`

	_, err = tx.Exec(query, uo.TxType, uo.Hash.Hex(), uo.Index, uo.From.Hex(), uo.To.Hex(), uo.Amount, uo.Timestamp, uo.BlockNum, uo.Node.Hex(), uo.TxType)
	if err != nil {
		return
	}

	return
}

// AddOutputBatch add new tx outputs to the database
func (s *Store) AddOutputBatch(txID int, values string) (rows int64, err error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()

	if !exists {
		return 0, errors.Errorf("Undefined transaction #%d", txID)
	}

	query := `INSERT INTO ` + dbshared.UtxoTable + ` (tx_type, hash, tx_index, address_from, address_to, address_node, amount, timestamp, blockId) VALUES ` + values

	res, err := tx.Exec(query)
	if err != nil {
		return
	}

	rows, err = res.RowsAffected()
	if err != nil {
		return
	}

	return
}

// SpendOutput delete output in the database
func (s *Store) SpendOutput(txID int, hash string, index uint32) (int64, error) {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()

	if !exists {
		return 0, errors.Errorf("SpendOutput: Undefined transaction #%d", txID)
	}

	query := `DELETE FROM ` + dbshared.UtxoTable + ` WHERE hash = ? AND tx_index = ?`

	res, err := tx.Exec(query, hash, index)
	if err != nil {
		return 0, err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	// rows can be equal to 0 when sync databases
	if rows != 1 && s.cfg.ShowFullStat {
		log.Debugf("Execute query: DELETE FROM %s WHERE hash = %s AND tx_index = %d", dbshared.UtxoTable, hash, index)
	}

	return rows, nil
}

// DeleteOutputs delete all outputs with given block number.
func (s *Store) DeleteOutputs(txID int, blockNum uint64) error {
	s.lock.RLock()
	tx, exists := s.tx[txID]
	s.lock.RUnlock()

	if !exists {
		return errors.Errorf("OutputDB.DeleteOutputs: Undefined transaction #%d", txID)
	}

	query := `DELETE FROM ` + dbshared.UtxoTable + ` WHERE blockId = ?`

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

	defer s.removeTx(txID)

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

	defer s.removeTx(txID)

	err = tx.Commit()
	if err != nil {
		return err
	}

	if s.cfg.ShowFullStat {
		log.Debugf("OutputDB.CommitTx: commit database tx id #%d.", txID)
	}

	return nil
}

// finishWriting prepare database for closing
func (s *Store) finishWriting() {
	s.lock.Lock()
	s.canCreateTx = false
	s.lock.Unlock()
}

// removeTx remove database transaction
func (s *Store) removeTx(id int) {
	s.lock.Lock()
	delete(s.tx, id)
	log.Debugf("OutputDB.removeTx: delete database tx id #%d", id)
	s.lock.Unlock()
}
