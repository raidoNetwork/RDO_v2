package utxo

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/fileutil"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"path/filepath"
	"sync"
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
	ShowFullStat bool
}

type Store struct {
	db           *sql.DB
	databasePath string
	ctx          context.Context

	tx          map[int]*sql.Tx
	txStatus    map[int]int
	txID        int
	canCreateTx bool
	lock        sync.RWMutex
	cfg         *Config
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
		txStatus:     make(map[int]int),
		cfg:          config,
		canCreateTx:  true,
	}

	if err := sqlDB.createSchema(); err != nil {
		return nil, err
	}

	return sqlDB, nil
}

// Close - close database connections
func (s *Store) Close() error {
	s.finishWriting()

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

// FindAllUTxO find all addresses' unspent outputs
func (s *Store) FindAllUTxO(addr string) (uoArr []*types.UTxO, err error) {
	query := `SELECT id, hash, tx_index, address_from, address_to, amount, spent, timestamp, blockId, tx_type FROM "` + utxoTable + `" WHERE address_to = ? AND spent = ?`

	start := time.Now()
	rows, err := s.db.Query(query, addr, common.UnspentTxO)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	if s.cfg.ShowFullStat {
		end := time.Since(start)
		log.Debugf("Get query result object in %s.", common.StatFmt(end))
	}

	uoArr = make([]*types.UTxO, 0)

	start = time.Now()
	showStat := true
	for rows.Next() {
		if showStat && s.cfg.ShowFullStat {
			log.Debugf("First row next in %s", common.StatFmt(time.Since(start)))
			showStat = false
		}

		startInner := time.Now()

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

		if s.cfg.ShowFullStat {
			endInner := time.Since(startInner)
			log.Debugf("Parse one row in %s.", common.StatFmt(endInner))
		}
	}

	if s.cfg.ShowFullStat {
		end := time.Since(start)
		log.Debugf("Get query parsed result in %s.", common.StatFmt(end))
	}

	return uoArr, nil
}

// FindLastBlockNum search max block num in the database.
func (s *Store) FindLastBlockNum() (num uint64, err error) {
	query := `SELECT IFNULL(MAX(blockId), 0) as maxBlockId FROM ` + utxoTable
	rows, err := s.db.Query(query)
	if err != nil {
		return
	}

	num = 0
	for rows.Next() {
		err = rows.Scan(&num)
		if err != nil {
			return
		}
	}

	return
}

func (s *Store) GetTotalAmount() (uint64, error) {
	query := `SELECT IFNULL(SUM(amount), 0) FROM ` + utxoTable + ` WHERE tx_type != ?`
	rows, err := s.db.Query(query, common.RewardTxType)
	if err != nil {
		return 0, nil
	}

	var sum uint64 = 0
	for rows.Next() {
		err = rows.Scan(&sum)
		if err != nil {
			return 0, err
		}
	}

	return sum, nil
}
