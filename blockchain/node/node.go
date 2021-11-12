package node

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/lansrv"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/kv"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/shared"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

var log = logrus.WithField("prefix", "node")

// RDONode defines a struct that handles the services running a random rdo chain
// full PoS node. It handles the lifecycle of the entire system and registers
// services to a service registry.
type RDONode struct {
	cliCtx   *cli.Context
	ctx      context.Context
	cancel   context.CancelFunc
	services *shared.ServiceRegistry
	lock     sync.RWMutex
	stop     chan struct{} // Channel to wait for termination notifications.
	db       db.Database
	outDB    db.OutputDatabase
}

// New creates a new node instance, sets up configuration options, and registers
// every required service to the node.
func New(cliCtx *cli.Context) (*RDONode, error) {
	/*if err := configureTracing(cliCtx); err != nil {
		return nil, err
	} */

	// load chain config from file if special file has been given
	configureChainConfig(cliCtx)

	// services map
	registry := shared.NewServiceRegistry()

	ctx, cancel := context.WithCancel(cliCtx.Context)
	rdo := &RDONode{
		cliCtx:   cliCtx,
		ctx:      ctx,
		cancel:   cancel,
		services: registry,
		stop:     make(chan struct{}),
	}

	// create database
	if err := rdo.startDB(cliCtx); err != nil {
		return nil, err
	}

	// utxo generator service
	if cliCtx.Bool(flags.LanSrv.Name) {
		if err := rdo.registerLanService(); err != nil {
			return nil, err
		}
	}

	return rdo, nil
}

func (r *RDONode) registerLanService() error {
	srv, err := lansrv.NewLanSrv(r.cliCtx, r.db, r.outDB)
	if err != nil {
		return err
	}

	if srv == nil {
		return errors.New("Empty Lan service implementation")
	}

	err = r.services.RegisterService(srv)
	if err != nil {
		log.Error("Can't start block gen service", err)
		return err
	}

	return nil
}

// Start the RDONode and kicks off every registered service.
func (r *RDONode) Start() {
	log.WithField("Version", version.Version()).Info("Starting raido node")

	r.lock.Lock()
	r.services.StartAll()
	stop := r.stop
	r.lock.Unlock()

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got interrupt, shutting down...")

		go r.Close()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.WithField("times", i-1).Info("Already shutting down, interrupt more to panic")
			}
		}
		panic("Panic closing the raido node")
	}()

	// Wait for stop channel to be closed.
	<-stop
}

// Close handles graceful shutdown of the system.
func (r *RDONode) Close() {
	r.lock.Lock()
	defer r.lock.Unlock()

	log.Info("Stopping raido node")
	r.services.StopAll()

	if err := r.db.Close(); err != nil {
		log.Errorf("Failed to close database: %v", err)
	}

	if err := r.outDB.Close(); err != nil {
		log.Errorf("Failed to close UTxO database: %v", err)
	}

	r.cancel()
	close(r.stop)
}

func (r *RDONode) startDB(cliCtx *cli.Context) error {
	baseDir := cliCtx.String(cmd.DataDirFlag.Name)
	dbPath := filepath.Join(baseDir, kv.RdoNodeDbDirName)
	clearDB := cliCtx.Bool(cmd.ClearDB.Name)
	forceClearDB := cliCtx.Bool(cmd.ForceClearDB.Name)

	log.WithField("database-path", dbPath).Info("Checking DB")

	d, err := db.NewDB(r.ctx, dbPath, &kv.Config{
		InitialMMapSize: cliCtx.Int(cmd.BoltMMapInitialSizeFlag.Name),
	})
	if err != nil {
		return err
	}

	clearDBConfirmed := false
	if clearDB && !forceClearDB {
		actionText := "This will delete your raido blockchain database stored in your data directory. " +
			"Your database backups will not be removed - do you want to proceed? (Y/N)"
		deniedText := "Database will not be deleted. No changes have been made."
		clearDBConfirmed, err = cmd.ConfirmAction(actionText, deniedText)
		if err != nil {
			return err
		}
	}

	if clearDBConfirmed || forceClearDB {
		log.Warning("Removing database")
		if err := d.Close(); err != nil {
			return errors.Wrap(err, "could not close db prior to clearing")
		}
		if err := d.ClearDB(); err != nil {
			return errors.Wrap(err, "could not clear database")
		}
		d, err = db.NewDB(r.ctx, dbPath, &kv.Config{
			InitialMMapSize: cliCtx.Int(cmd.BoltMMapInitialSizeFlag.Name),
		})
		if err != nil {
			return errors.Wrap(err, "could not create new database")
		}
	}

	r.db = d

	// Prepare SQL database config
	SQLCfg := db.SQLConfig{
		ShowFullStat: cliCtx.Bool(flags.LanSrvStat.Name),
		ConfigPath:   cliCtx.String(cmd.SQLConfigPath.Name),
		DataDir:      dbPath,
	}
	outDB, err := db.NewUTxODB(r.ctx, dbPath, &SQLCfg)
	if err != nil {
		return err
	}

	r.outDB = outDB

	return nil
}
