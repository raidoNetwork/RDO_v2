package node

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/rdochain"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/kv"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/gateway"
	"github.com/raidoNetwork/RDO_v2/rpc"
	"github.com/raidoNetwork/RDO_v2/shared"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

const (
	statusNotReady = iota
	statusReady
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
	kvStore       db.Database
	outDB    db.OutputDatabase

	status int // Node ready status
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
		status:   statusNotReady,
	}

	// create database
	if err := rdo.startDB(cliCtx); err != nil {
		return nil, err
	}

	// register chain service
	if err := rdo.registerChainService(); err != nil {
		log.Error("Error register ChainService.")
		return nil, err
	}

	// register RPC service
	if err := rdo.registerRPCservice(); err != nil {
		log.Error("Error register RPC service.")
		return nil, err
	}

	// register gRPC gateway service
	if err := rdo.registerGatewayService(); err != nil {
		log.Error("Error register gRPC gateway.")
		return nil, err
	}

	return rdo, nil
}

func (r *RDONode) registerChainService() error {
	srv, err := rdochain.NewService(r.cliCtx, r.kvStore, r.outDB)
	if err != nil {
		return err
	}

	return r.services.RegisterService(srv)
}

func (r *RDONode) registerRPCservice() error {
	var chainService *rdochain.Service
	err := r.services.FetchService(&chainService)
	if err != nil {
		return err
	}

	host := r.cliCtx.String(flags.RPCHost.Name)
	port := r.cliCtx.String(flags.RPCPort.Name)

	srv := rpc.NewService(r.ctx, &rpc.Config{
		Host:               host,
		Port:               port,
		ChainService:       chainService,
		AttestationService: chainService, // TODO change it to another service in future
		MaxMsgSize:         1 << 22,
	})

	return r.services.RegisterService(srv)
}

func (r *RDONode) registerGatewayService() error {
	host := r.cliCtx.String(flags.GRPCGatewayHost.Name)
	port := r.cliCtx.Int(flags.GRPCGatewayPort.Name)
	rpcPort := r.cliCtx.Int(flags.RPCPort.Name)
	remoteAddr := fmt.Sprintf("%s:%d", host, rpcPort)
	endpoint := fmt.Sprintf("%s:%d", host, port)
	allowedOrigins := strings.Split(r.cliCtx.String(flags.GPRCGatewayCorsDomain.Name), ",")

	gatewayConfig := gateway.DefaultConfig()

	srv := gateway.NewService(r.ctx,
		remoteAddr,
		endpoint,
		[]gateway.PbMux{gatewayConfig.PbMux},
		gatewayConfig.Handler).WithAllowedOrigins(allowedOrigins)

	return r.services.RegisterService(srv)
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

	if err := r.kvStore.Close(); err != nil {
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
	dbType := cliCtx.String(cmd.SQLType.Name)

	log.WithField("database-path", dbPath).Info("Checking DB")

	// Init key value database
	kvStore, err := db.NewDB(r.ctx, dbPath, &kv.Config{
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
		if err := kvStore.Close(); err != nil {
			return errors.Wrap(err, "could not close db prior to clearing")
		}
		if err := kvStore.ClearDB(); err != nil {
			return errors.Wrap(err, "could not clear database")
		}
		kvStore, err = db.NewDB(r.ctx, dbPath, &kv.Config{
			InitialMMapSize: cliCtx.Int(cmd.BoltMMapInitialSizeFlag.Name),
		})
		if err != nil {
			return errors.Wrap(err, "could not create new KV database")
		}
	}

	// save database instance to the node
	r.kvStore = kvStore

	// Prepare SQL database config
	SQLCfg := db.SQLConfig{
		ShowFullStat: cliCtx.Bool(flags.SrvStat.Name),
		ConfigPath:   cliCtx.String(cmd.SQLConfigPath.Name),
		DataDir:      dbPath,
	}

	// Init SQL database
	sqlStore, err := db.NewUTxODB(r.ctx, dbType, &SQLCfg)
	if err != nil {
		return errors.Wrap(err, "could not create new SQL database")
	}

	// Save database instance to the node
	r.outDB = sqlStore

	return nil
}
