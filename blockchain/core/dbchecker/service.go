package dbchecker

import (
	"encoding/binary"
	"encoding/hex"
	"github.com/sirupsen/logrus"
	"rdo_draft/blockchain/db"
	"rdo_draft/shared/keystore"
	"time"
)

var log = logrus.WithField("prefix", "db service")

type DBService struct {
	timeout time.Duration
	stop    chan struct{}
	db      db.Database
}

func (d *DBService) Start() {
	timer := time.NewTicker(d.timeout)

	for range timer.C {
		log.Info("DB work...")
		d.worker()
	}
}

func (d *DBService) worker() {
	rows, err := d.db.ReadAllData()
	if err != nil {
		log.Error("Error reading data", err)
		return
	}

	for i := len(rows) - 1; i > -1; i-- {
		v := rows[i]
		log.Infof("Read row %d with value %s.", v.Timestamp, hex.EncodeToString(v.Hash))
	}

	// write data to database
	stamp := time.Now().UnixNano()
	key := make([]byte, 8)

	binary.BigEndian.PutUint64(key, uint64(stamp))
	h := keystore.Hash(key)
	data := h[:]

	log.Warnf("Write row %d with value %s.", stamp, hex.EncodeToString(data))

	err = d.db.SaveData(key, data)
	if err != nil {
		log.Error("Error saving data", err)
		return
	}
}

func (d *DBService) Stop() error {
	close(d.stop)

	return nil
}

func (d *DBService) Status() error {
	return nil
}

func NewDBService(db db.Database) *DBService {
	s := &DBService{
		db:      db,
		timeout: 5 * time.Second,
		stop:    make(chan struct{}),
	}

	return s
}
