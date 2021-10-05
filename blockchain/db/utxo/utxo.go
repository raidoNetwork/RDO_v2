package utxo

import (
	"context"
	"encoding/hex"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"rdo_draft/shared/common"
	"rdo_draft/shared/fileutil"
	"rdo_draft/shared/types"
	"time"

	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

const (
	utxoPath  = "utxo"
	utxoFname = "outputs.db"
)

var (
	ErrBadUO = errors.New("Wrong utxo given. Please check that all fields have value.")

	log = logrus.WithField("prefix", "OutputDB")
)

type Config struct {
}

type Store struct {
	db           *sql.DB
	databasePath string
	ctx          context.Context

	tx   map[int]*sql.Tx
	txID int
}

func NewUTxOStore(ctx context.Context, dirPath string, config *Config) (*Store, error) {
	dbPath := filepath.Join(dirPath, utxoPath)
	hasDir, err := fileutil.HasDir(dbPath)
	if err != nil {
		return nil, err
	}
	if !hasDir {
		if err := fileutil.MkdirAll(dbPath); err != nil {
			return nil, err
		}
	}

	fpath := filepath.Join(dbPath, utxoFname)
	db, err := sql.Open("sqlite3", fpath)
	if err != nil {
		return nil, err
	}

	sqlDB := &Store{
		db:           db,
		databasePath: dirPath,
		ctx:          ctx,
		txID:         1,
		tx:           make(map[int]*sql.Tx),
	}

	if err := sqlDB.createSchema(); err != nil {
		return nil, err
	}

	return sqlDB, nil
}

// Close - close database connections
func (s *Store) Close() error {
	return s.db.Close()
}

// createSchema generates needed database structure if not exist.
func (s *Store) createSchema() error {
	stmnt, err := s.db.Prepare(utxoSchema)
	if err != nil {
		return err
	}

	defer stmnt.Close()

	_, err = stmnt.Exec()
	if err != nil {
		return err
	}

	return nil
}

// DatabasePath at which this database writes files.
func (s *Store) DatabasePath() string {
	return s.databasePath
}

// AddOutput stores given output in the database
func (s *Store) AddOutput(uo *types.UTxO) (id int64, err error) {
	query := `INSERT INTO "` + utxoTable + `" (tx_type, hash, tx_index, address_from, address_to, amount, timestamp, spent, blockId) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := s.db.Prepare(query)
	if err != nil {
		return
	}

	defer stmt.Close()

	hash := hex.EncodeToString(uo.Hash)
	from := hex.EncodeToString(uo.From)
	to := hex.EncodeToString(uo.To)

	res, err := stmt.Exec(uo.TxType, hash, uo.Index, from, to, uo.Amount, time.Now().UnixNano(), common.UnspentTxO, uo.BlockNum)
	if err != nil {
		return
	}

	id, err = res.LastInsertId()
	if err != nil {
		return
	}

	return
}

// SpendOutput mark output with given id as spent output in the database.
func (s *Store) SpendOutput(id uint64) error {
	query := `UPDATE ` + utxoTable + ` SET spent = ? WHERE id = ?`
	stmt, err := s.db.Prepare(query)
	defer stmt.Close()

	if err != nil {
		return err
	}

	if id == 0 {
		return ErrBadUO
	}

	_, err = stmt.Exec(common.SpentTxO, id)
	if err != nil {
		return err
	}

	return nil
}

// TODO rewrite it
func (s *Store) HealthCheck() (lastId uint64, err error) {
	query := `SELECT id, hash, tx_index, address_from, address_to, amount, spent, timestamp, blockId, tx_type FROM "` + utxoTable + `" ORDER BY "id" DESC LIMIT 20`

	rows, err := s.db.Query(query)
	if err != nil {
		return
	}

	var uo types.UTxO
	for rows.Next() {
		var hash, from, to string

		uo = types.UTxO{
			Hash:      make([]byte, 0, 32),
			Index:     0,
			From:      make([]byte, 0, 32),
			To:        make([]byte, 0, 32),
			Amount:    0,
			Spent:     common.UnspentTxO,
			ID:        0,
			Timestamp: 0,
			BlockNum:  0,
			TxType:    common.NormalTxType,
		}

		err = rows.Scan(&uo.ID, &hash, &uo.Index, &from, &to, &uo.Amount, &uo.Spent, &uo.Timestamp, &uo.BlockNum, &uo.TxType)
		if err != nil {
			return
		}

		uo.Hash, err = hex.DecodeString(hash)
		if err != nil {
			return
		}

		uo.From, err = hex.DecodeString(from)
		if err != nil {
			return
		}

		uo.To, err = hex.DecodeString(to)
		if err != nil {
			return
		}

		// save last id
		if lastId == 0 {
			lastId = uo.ID
		}

		log.Info(uo.ToString())
	}

	return
}

func (s *Store) AddOutputBlockNum(n uint64, id int) error {
	query := `UPDATE ` + utxoTable + ` SET blockId = ? WHERE id = ?`
	stmt, err := s.db.Prepare(query)
	defer stmt.Close()

	if err != nil {
		return err
	}

	_, err = stmt.Exec(n, id)
	if err != nil {
		return err
	}

	return nil
}

// FindGenesisOutput tries to get output from genesis block on user with given address.
// if UTxO found return it otherwise return nil
func (s *Store) FindGenesisOutput(addr string) (uo *types.UTxO, err error) {
	query := `SELECT id, hash, tx_index, address_from, address_to, amount, spent, timestamp, blockId FROM "` + utxoTable + `" WHERE address_to = ? AND tx_type = ?`
	row := s.db.QueryRow(query, addr, common.GenesisTxType)

	err = row.Err()
	if err != nil {
		return nil, err
	}

	var hash, from, to string
	var id, spent, blockNum, amount, timestamp uint64
	var index uint32

	err = row.Scan(&id, &hash, &index, &from, &to, &amount, &spent, &timestamp, &blockNum)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return
	}

	uo, err = types.NewUTxOFull(id, hash, from, to, index, amount, blockNum, spent, timestamp, common.GenesisTxType)
	if err != nil {
		return nil, err
	}

	return uo, nil
}

// FindAllUTxO find all addresses' unspent outputs
func (s *Store) FindAllUTxO(addr string) (uoArr []*types.UTxO, err error) {
	query := `SELECT id, hash, tx_index, address_from, address_to, amount, spent, timestamp, blockId, tx_type FROM "` + utxoTable + `" WHERE address_to = ? AND spent = ?`

	rows, err := s.db.Query(query, addr, common.UnspentTxO)
	if err != nil {
		return nil, err
	}

	uoArr = make([]*types.UTxO, 0)

	for rows.Next() {
		var hash, from, to string
		var id, spent, blockNum, amount, timestamp uint64
		var index uint32
		var typev int

		err = rows.Scan(&id, &hash, &index, &from, &to, &amount, &spent, &timestamp, &blockNum, &typev)
		if err != nil {
			return
		}

		uo, err := types.NewUTxOFull(id, hash, from, to, index, amount, blockNum, spent, timestamp, typev)
		if err != nil {
			return nil, err
		}

		uoArr = append(uoArr, uo)
	}

	return uoArr, nil
}

func (s *Store) CreateTx() (int, error) {
	ctx := context.Background()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}

	s.tx[s.txID] = tx

	id := s.txID
	s.txID++

	return id, nil
}

func (s *Store) RollbackTx(txID int) (err error) {
	tx, exists := s.tx[txID]
	if !exists {
		return errors.Errorf("Undefined transaction #%d", txID)
	}

	return tx.Rollback()
}

func (s *Store) CommitTx(txID int) (err error) {
	tx, exists := s.tx[txID]
	if !exists {
		return errors.Errorf("Undefined transaction #%d", txID)
	}

	return tx.Commit()
}

func (s *Store) SpendOutputWithTx(txID int, id uint64) error {
	tx, exists := s.tx[txID]
	if !exists {
		return errors.Errorf("Undefined transaction #%d", txID)
	}

	query := `UPDATE ` + utxoTable + ` SET spent = ? WHERE id = ?`

	stmt, err := tx.Prepare(query)
	defer stmt.Close()

	if err != nil {
		return err
	}

	if id == 0 {
		return ErrBadUO
	}

	_, err = stmt.Exec(common.SpentTxO, id)
	if err != nil {
		return err
	}

	return nil
}

func (s *Store) AddOutputWithTx(txID int, uo *types.UTxO) (id int64, err error) {
	tx, exists := s.tx[txID]
	if !exists {
		return 0, errors.Errorf("Undefined transaction #%d", txID)
	}

	query := `INSERT INTO "` + utxoTable + `" (tx_type, hash, tx_index, address_from, address_to, amount, timestamp, spent, blockId) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := tx.Prepare(query)
	if err != nil {
		return
	}

	defer stmt.Close()

	hash := hex.EncodeToString(uo.Hash)
	from := hex.EncodeToString(uo.From)
	to := hex.EncodeToString(uo.To)

	res, err := stmt.Exec(uo.TxType, hash, uo.Index, from, to, uo.Amount, time.Now().UnixNano(), common.UnspentTxO, uo.BlockNum)
	if err != nil {
		return
	}

	id, err = res.LastInsertId()
	if err != nil {
		return
	}

	return
}
