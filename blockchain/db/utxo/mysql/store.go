package mysql

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/iface"
	"github.com/sirupsen/logrus"
	"os"
)

const (
	DatabaseName = "utxo"
)

var log = logrus.WithField("prefix", "MySQL")

func NewStore(config *iface.SQLConfig) (*sql.DB, string, error) {
	path := config.ConfigPath
	if path == "" {
		return nil, "", errors.New("Empty database cfg path.")
	}

	err := godotenv.Load(path)
	if err != nil {
		return nil, "", errors.Errorf("Error loading .env file %s.", path)
	}

	var uname, pass string
	uname = os.Getenv("DB_USER")
	pass = os.Getenv("DB_PASS")

	log.Info("Database config was parsed successfully.")

	db, err := sql.Open("mysql", uname+":"+pass+"@tcp(127.0.0.1:3306)/"+DatabaseName)
	if err != nil {
		return nil, "", err
	}

	return db, utxoSchema, nil
}
