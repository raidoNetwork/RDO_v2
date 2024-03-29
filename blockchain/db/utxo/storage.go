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
	"github.com/raidoNetwork/RDO_v2/utils/slice"
	"github.com/sirupsen/logrus"
	"os"
	"sync"
)

var log = logrus.WithField("prefix", "OutputDB")

func NewStore(ctx context.Context, config *iface.SQLConfig) (*Store, error) {
	path := config.ConfigPath
	if path == "" {
		return nil, errors.New("Empty database cfg path.")
	}

	err := godotenv.Load(path)
	if err != nil {
		return nil, errors.Errorf("Error loading .env file %s. Error message: %s", path, err)
	}

	var uname, pass, host, port, dbname string
	uname = os.Getenv("DB_USER")
	pass = os.Getenv("DB_PASS")
	host = os.Getenv("DB_HOST")
	port = os.Getenv("DB_PORT")
	dbname = os.Getenv("DB_NAME")

	dataSource := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", uname, pass, host, port, dbname)
	log.Debugf("SQL database server: %s:%s", host, port)

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
		canCreateTx:  true,
		schema:       utxoSchema,
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

	schema string
}

// Close - close database connections
func (s *Store) Close() error {
	s.finishWriting()

	return s.db.Close()
}

// createSchema generates needed database structure if not exist.
func (s *Store) createSchema() error {
	_, err := s.db.Exec(s.schema)
	return err
}

// DatabasePath at which this database writes files.
func (s *Store) DatabasePath() string {
	return s.databasePath
}

// FindAllUTxO find all addresses' unspent outputs
func (s *Store) FindAllUTxO(addr string) (uoArr []*types.UTxO, err error) {
	query := "WHERE address_to = ? AND address_node = ?"
	return s.getOutputsList(query, addr, "")
}

// FindLastBlockNum search max block num in the database.
func (s *Store) FindLastBlockNum() (num uint64, err error) {
	query := fmt.Sprintf("SELECT IFNULL(MAX(block_id), 0) as maxBlockId FROM %s", dbshared.UtxoTable)
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
	query := fmt.Sprintf("SELECT IFNULL(SUM(amount), 0) FROM %s", dbshared.UtxoTable)
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
	query := `WHERE (tx_type = ? OR tx_type = ?) AND address_node != ""`
	return s.getOutputsList(query, common.StakeTxType, common.UnstakeTxType)
}

func (s *Store) FindValidatorStakeDeposits() (uoArr []*types.UTxO, err error) {
	query := "WHERE (tx_type = ? OR tx_type = ?) AND address_node = ?"
	return s.getOutputsList(query, common.StakeTxType, common.UnstakeTxType, common.BlackHoleAddress)
}

// FindStakeDepositsOfAddress shows actual stake deposits of given address
// and return list of deposit outputs.
func (s *Store) FindStakeDepositsOfAddress(address string, node string) ([]*types.UTxO, error) {
	nodeAddress := common.BlackHoleAddress
	if node != "" {
		nodeAddress = node
	}
	nodePlaceholder := "= ?"
	arguments := []interface{}{
		common.StakeTxType,
		common.UnstakeTxType,
		address,
	}
	if node == "all" {
		nodePlaceholder = `!= ""`
	} else {
		arguments = slice.Insert(arguments, nodeAddress, 2)
	}

	query := "WHERE (tx_type = ? OR tx_type = ? ) AND address_node " + nodePlaceholder + " AND address_to = ?"
	return s.getOutputsList(query, arguments...)
}

// getOutputsList return outputs list with given query and params.
func (s *Store) getOutputsList(condQuery string, params ...interface{}) (uoArr []*types.UTxO, err error) {
	query := fmt.Sprintf(`SELECT id,
					hash,
					tx_index, 
					address_from, 
					address_to, 
					address_node, 
					amount, 
					timestamp, 
					block_id, 
					tx_type FROM %s %s`, dbshared.UtxoTable, condQuery)

	rows, err := s.db.Query(query, params...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	uoArr = make([]*types.UTxO, 0)

	var hash, from, to, node string
	var id, blockNum, amount, timestamp uint64
	var index, typev uint32
	for rows.Next() {
		err = rows.Scan(&id, &hash, &index, &from, &to, &node, &amount, &timestamp, &blockNum, &typev)
		if err != nil {
			return
		}

		uo := types.NewUTxOFull(id, hash, from, to, node, index, amount, blockNum, timestamp, typev)
		uoArr = append(uoArr, uo)
	}

	return uoArr, nil
}

func (s *Store) ClearDatabase() error {
	query := fmt.Sprintf("DROP TABLE %s", dbshared.UtxoTable)
	_, err := s.db.Exec(query)
	return err
}