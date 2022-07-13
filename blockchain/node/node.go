package node

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/core"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/attestation"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/rdochain"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/settings"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/kv"
	rsync "github.com/raidoNetwork/RDO_v2/blockchain/sync"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/gateway"
	"github.com/raidoNetwork/RDO_v2/generator"
	"github.com/raidoNetwork/RDO_v2/metrics"
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/rpc"
	"github.com/raidoNetwork/RDO_v2/shared"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/params"
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

var log = logrus.WithField("prefix", "node")

const maxMsgSize =  1 << 22

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
	kvStore  db.Database
	outDB    db.OutputDatabase
	stateFeed events.Bus
	blockFeed events.Bus
	txFeed	  events.Bus
}

// New creates a new node instance, sets up configuration options, and registers
// every required service to the node.
func New(cliCtx *cli.Context) (*RDONode, error) {
	// Setup default config
	params.UseMainnetConfig()

	// load chain config from file if special file has been given
	settings.ConfigureChainConfig(cliCtx)

	// services map
	registry := shared.NewServiceRegistry()

	// create slot ticker
	slot.NewSlotTicker()

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

	// register P2P service
	if err := rdo.registerP2P(); err != nil {
		log.Error("Error register P2P service.")
		return nil, errors.Wrap(err, "P2P error")
	}

	// register blockchain service
	if err := rdo.registerBlockchainService(); err != nil {
		log.Error("Error register BlockchainService.")
		return nil, err
	}

	// register attestation service
	if err := rdo.registerAttestationService(); err != nil {
		log.Error("Error register BlockchainService.")
		return nil, err
	}

	// register chain service
	if err := rdo.registerCoreService(); err != nil {
		log.Error("Error register ChainService.")
		return nil, err
	}

	// register sync service
	if err := rdo.registerSyncService(); err != nil {
		log.Error("Error register sync service.")
		return nil, errors.Wrap(err, "sync error")
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

	if cliCtx.Bool(flags.EnableMetrics.Name) {
		if err := rdo.registerMetricsService(); err != nil {
			log.Error("Error register metrics endpoint.")
			return nil, err
		}
	}


	return rdo, nil
}

func (r *RDONode) registerCoreService() error {
	var blockchainService *rdochain.Service
	err := r.services.FetchService(&blockchainService)
	if err != nil {
		return err
	}

	var attestationService *attestation.Service
	err = r.services.FetchService(&attestationService)
	if err != nil {
		return err
	}

	cfg := core.Config{
		BlockForger: blockchainService,
		AttestationPool: attestationService,
		BlockFeed: r.BlockFeed(),
		StateFeed: r.StateFeed(),
		Context:   r.ctx,
	}
	srv, err := core.NewService(r.cliCtx, &cfg)
	if err != nil {
		return err
	}

	return r.services.RegisterService(srv)
}

func (r *RDONode) registerRPCservice() error {
	var blockchainService *rdochain.Service
	err := r.services.FetchService(&blockchainService)
	if err != nil {
		return err
	}

	var attestationService *attestation.Service
	err = r.services.FetchService(&attestationService)
	if err != nil {
		return err
	}

	host := r.cliCtx.String(flags.RPCHost.Name)
	port := r.cliCtx.String(flags.RPCPort.Name)

	genService := generator.NewService(blockchainService)

	srv := rpc.NewService(r.ctx, &rpc.Config{
		Host:               host,
		Port:               port,
		ChainService:       blockchainService,
		AttestationService: attestationService,
		GeneratorService:   genService,
		MaxMsgSize:         maxMsgSize,
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
		maxMsgSize,
		[]gateway.PbMux{gatewayConfig.PbMux},
		gatewayConfig.Handler).WithAllowedOrigins(allowedOrigins)

	return r.services.RegisterService(srv)
}

func (r *RDONode) registerBlockchainService() error {
	srv, err := rdochain.NewService(r.kvStore, r.outDB, r.StateFeed())
	if err != nil {
		return err
	}

	return r.services.RegisterService(srv)
}

func (r *RDONode) registerAttestationService() error {
	var blockchainService *rdochain.Service
	err := r.services.FetchService(&blockchainService)
	if err != nil {
		return err
	}

	enableStats := r.cliCtx.Bool(flags.EnableMetrics.Name)
	cfg := attestation.Config{
		TxFeed:        r.TxFeed(),
		StateFeed:     r.StateFeed(),
		EnableMetrics: enableStats,
		Blockchain:    blockchainService,
	}
	srv, err := attestation.NewService(r.ctx, &cfg)
	if err != nil {
		return err
	}

	return r.services.RegisterService(srv)
}

func (r *RDONode) registerP2P() error {
	cfg := p2p.Config{
		Host: r.cliCtx.String(flags.P2PHost.Name),
		Port: r.cliCtx.Int(flags.P2PPort.Name),
		BootstrapNodes: r.cliCtx.StringSlice(flags.P2PBootstrapNodes.Name),
		DataDir: r.cliCtx.String(cmd.DataDirFlag.Name),
		StateFeed: r.StateFeed(),
		EnableNAT: r.cliCtx.Bool(flags.P2PEnableNat.Name),
	}
	srv, err := p2p.NewService(r.ctx, &cfg)
	if err != nil {
		return err
	}

	return r.services.RegisterService(srv)
}

func (r *RDONode) registerSyncService() error {
	var p2pSrv *p2p.Service
	err := r.services.FetchService(&p2pSrv)
	if err != nil {
		return err
	}

	var blockchainService *rdochain.Service
	err = r.services.FetchService(&blockchainService)
	if err != nil {
		return err
	}

	var coreService *core.Service
	err = r.services.FetchService(&coreService)
	if err != nil {
		return err
	}

	cfg := rsync.Config{
		BlockFeed: r.BlockFeed(),
		TxFeed: r.TxFeed(),
		StateFeed: r.StateFeed(),
		P2P: p2pSrv,
		Storage: coreService,
		Blockchain: blockchainService,
	}
	srv := rsync.NewService(r.ctx, &cfg)

	return r.services.RegisterService(srv)
}

func (r *RDONode) registerMetricsService() error {
	host := r.cliCtx.String(flags.MetricsHost.Name)
	port := r.cliCtx.Int(flags.MetricsPort.Name)
	endpoint := fmt.Sprintf("%s:%d", host, port)
	srv := metrics.New(endpoint, r.services)

	return r.services.RegisterService(srv)
}

// Start the RDONode and kicks off every registered service.
func (r *RDONode) Start() {
	log.WithFields(logrus.Fields{
		"version": version.Version(),
	}).Info("Starting raido node")

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

	log.Info("Raido node shutdown...")

	slot.Ticker().Stop()
	log.Info("Slot ticker stopped.")

	r.services.StopAll()
	r.cancel()

	if err := r.kvStore.Close(); err != nil {
		log.Errorf("Failed to close KV database: %v", err)
	}

	if err := r.outDB.Close(); err != nil {
		log.Errorf("Failed to close UTxO database: %v", err)
	}

	close(r.stop)
}

// startDB create database files and prepare databases schema.
// Also clear database if certain flags given.
func (r *RDONode) startDB(cliCtx *cli.Context) error {
	baseDir := cliCtx.String(cmd.DataDirFlag.Name)
	dbPath := filepath.Join(baseDir, kv.RdoNodeDbDirName)
	clearDB := cliCtx.Bool(cmd.ClearDB.Name)
	forceClearDB := cliCtx.Bool(cmd.ForceClearDB.Name)

	log.WithField("database-path", dbPath).Info("Starting databases...")

	// Init key value database
	kvStore, err := db.NewDB(r.ctx, dbPath)
	if err != nil {
		return err
	}

	// Prepare SQL database config
	SQLCfg := db.SQLConfig{
		ConfigPath:   cliCtx.String(cmd.SQLConfigPath.Name),
		DataDir:      dbPath,
	}

	// Init SQL database
	sqlStore, err := db.NewUTxODB(r.ctx, &SQLCfg)
	if err != nil {
		return errors.Wrap(err, "could not create new SQL database")
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
			return errors.Wrap(err, "could not clear KV database")
		}
		if err := sqlStore.ClearDatabase(); err != nil {
			return errors.Wrap(err, "could not clear Outputs database")
		}
		kvStore, err = db.NewDB(r.ctx, dbPath)
		if err != nil {
			return errors.Wrap(err, "could not create new KV database")
		}
	}

	// save database instance to the node
	r.kvStore = kvStore
	r.outDB = sqlStore

	return nil
}

func (r *RDONode) BlockFeed() *events.Bus {
	return &r.blockFeed
}

func (r *RDONode) StateFeed() *events.Bus {
	return &r.stateFeed
}

func (r *RDONode) TxFeed() *events.Bus {
	return &r.txFeed
}
