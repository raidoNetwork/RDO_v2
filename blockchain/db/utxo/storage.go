package utxo

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/iface"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/utxo/dbshared"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"os"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "OutputDB")

func NewStore(ctx context.Context, dbType string, config *iface.SQLConfig) (*Store, error) {
	path := config.ConfigPath
	if path == "" {
		return nil, errors.New("Empty database cfg path.")
	}

	err := godotenv.Load(path)
	if err != nil {
		return nil,  errors.Errorf("Error loading .env file %s.", path)
	}

	var uname, pass, host, port string
	uname = os.Getenv("DB_USER")
	pass = os.Getenv("DB_PASS")
	host = os.Getenv("DB_HOST")
	port = os.Getenv("DB_PORT")

	dataSource := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", uname, pass, host, port, dbshared.DBname)

	log.Info("Database config was parsed successfully.")

	db, err := sql.Open("mysql", dataSource)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	str := &Store{
		db:           db,
		databasePath: config.DataDir,
		ctx:          ctx,
		txID:         1,
		tx:           make(map[int]*sql.Tx),
		cfg:          config,
		canCreateTx:  true,
		schema:       utxoSchema,
		dbType:       dbType,
	}

	if err := str.createSchema(); err != nil {
		return nil, err
	}

	return str, nil
}

type Store struct {
	db           *sql.DB
	databasePath string
	ctx          context.Context
	tx           map[int]*sql.Tx
	txID         int
	canCreateTx  bool
	lock         sync.RWMutex
	cfg          *iface.SQLConfig

	schema string
	dbType string
}

// Close - close database connections
func (s *Store) Close() error {
	s.finishWriting()

	return s.db.Close()
}

// createSchema generates needed database structure if not exist.
func (s *Store) createSchema() error {
	_, err := s.db.Exec(s.schema)
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
	query := `WHERE address_to = ? AND spent = ? AND address_node = ?`
	return s.getOutputsList(query, addr, common.UnspentTxO, "")
}

// FindLastBlockNum search max block num in the database.
func (s *Store) FindLastBlockNum() (num uint64, err error) {
	query := `SELECT IFNULL(MAX(blockId), 0) as maxBlockId FROM ` + dbshared.UtxoTable
	rows, err := s.db.Query(query)
	if err != nil {
		return
	}

	defer rows.Close()

	num = 0
	for rows.Next() {
		err = rows.Scan(&num)
		if err != nil {
			return
		}
	}

	return
}

// GetTotalAmount return sum amount of network
func (s *Store) GetTotalAmount() (uint64, error) {
	query := `SELECT IFNULL(SUM(amount), 0) FROM ` + dbshared.UtxoTable
	rows, err := s.db.Query(query)
	if err != nil {
		return 0, nil
	}

	defer rows.Close()

	var sum uint64 = 0
	for rows.Next() {
		err = rows.Scan(&sum)
		if err != nil {
			return 0, err
		}
	}

	return sum, nil
}

// FindStakeDeposits shows all actual stake deposits and return list of deposit outputs.
func (s *Store) FindStakeDeposits() (uoArr []*types.UTxO, err error) {
	query := `WHERE tx_type = ? AND address_node = ?`
	return s.getOutputsList(query, common.StakeTxType, common.BlackHoleAddress)
}

// FindStakeDepositsOfAddress shows actual stake deposits of given address
// and return list of deposit outputs.
func (s *Store) FindStakeDepositsOfAddress(address string) ([]*types.UTxO, error) {
	query := `WHERE tx_type = ? AND address_node = ? AND address_to = ?`
	return s.getOutputsList(query, common.StakeTxType, common.BlackHoleAddress, address)
}

// getOutputsList return outputs list with given query and params.
func (s *Store) getOutputsList(query string, params ...interface{}) (uoArr []*types.UTxO, err error) {
	start := time.Now()
	prefix := `SELECT id, hash, tx_index, address_from, address_to, address_node, amount, spent, timestamp, blockId, tx_type FROM ` + dbshared.UtxoTable + ` `
	rows, err := s.db.Query(prefix+query, params...)
	if err != nil {
		return nil, err
	}

	if s.cfg.ShowFullStat {
		end := time.Since(start)
		log.Debugf("Get query object in %s.", common.StatFmt(end))
	}

	defer rows.Close()

	uoArr = make([]*types.UTxO, 0)

	start = time.Now()

	var hash, from, to, node string
	var id, spent, blockNum, amount, timestamp uint64
	var index, typev uint32
	for rows.Next() {
		err = rows.Scan(&id, &hash, &index, &from, &to, &node, &amount, &spent, &timestamp, &blockNum, &typev)
		if err != nil {
			return
		}

		uo, err := types.NewUTxOFull(id, hash, from, to, node, index, amount, blockNum, spent, timestamp, typev)
		if err != nil {
			return nil, err
		}

		uoArr = append(uoArr, uo)
	}

	if s.cfg.ShowFullStat {
		end := time.Since(start)
		log.Debugf("Get query parsed result in %s.", common.StatFmt(end))
	}

	return uoArr, nil
}
