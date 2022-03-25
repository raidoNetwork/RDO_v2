package core

import (
	"context"
	"fmt"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/miner"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "core")

type blockEvent struct {
	data *prototype.Block
	isLocal bool
}

type Config struct{
	BlockForger consensus.BlockForger
	AttestationPool consensus.AttestationPool
	StateFeed *events.Bus
	BlockFeed *events.Bus
}

// NewService creates new CoreService
func NewService(cliCtx *cli.Context, cfg *Config) (*Service, error) {
	statFlag := cliCtx.Bool(flags.SrvStat.Name)
	debugStatFlag := cliCtx.Bool(flags.SrvDebugStat.Name)
	dataDir := cliCtx.String(cmd.DataDirFlag.Name)

	netCfg := params.RaidoConfig()

	// TODO rework validator address loading
	proposer, err := keystore.NewValidatorAccountFromFile(dataDir)
	if err != nil {
		return nil, err
	}

	var msg string
	if proposer.Key() != nil {
		msg = fmt.Sprintf("Master node with proposer %s", proposer.Addr().Hex())
	} else{
		msg = "Slave node"
	}
	log.Info(msg)

	minerCfg := &miner.MinerConfig{
		ShowStat:     statFlag,
		ShowFullStat: debugStatFlag,
		BlockSize:    netCfg.BlockSize,
		Proposer:     proposer,
	}

	// new block miner
	forger := miner.NewMiner(cfg.BlockForger, cfg.AttestationPool, minerCfg)

	ctx, finish := context.WithCancel(context.Background())

	srv := &Service{
		cliCtx:     cliCtx,
		ctx:        ctx,
		cancelFunc: finish,
		miner:      forger,
		proposer:   proposer,
		bc:         cfg.BlockForger,

		stop:       make(chan struct{}),
		ticker:     slot.Ticker(),

		// flags
		fullStatFlag: statFlag,
		expStatFlag:  debugStatFlag,

		// events
		blockEvent: make(chan blockEvent),
		stateEvent: make(chan state.State),

		// feeds
		blockFeed: cfg.BlockFeed,
	}

	// subscribe on events
	cfg.StateFeed.Subscribe(srv.stateEvent)
	cfg.BlockFeed.Subscribe(srv.blockEvent)

	return srv, nil
}

// Service implements blockchain service for blockchain update, read and creating new blocks.
type Service struct {
	cliCtx       *cli.Context
	ctx          context.Context
	cancelFunc   context.CancelFunc
	statusErr    error
	stop         chan struct{}
	bc			 consensus.BlockForger
	proposer 	 *keystore.ValidatorAccount

	miner *miner.Miner            // block miner
	ticker *slot.SlotTicker

	// flags
	fullStatFlag bool
	expStatFlag  bool

	mu sync.Mutex

	// events
	stateEvent chan state.State
	blockEvent chan blockEvent

	blockFeed events.Emitter
}

// Start service work
func (s *Service) Start() {
	s.waitInitialized()

	// start slot ticker
	genesisTime := time.Unix(0, int64(s.bc.GetGenesis().Timestamp))
	slot.Ticker().Start(genesisTime)

	log.Warn("Start Core service.")

	// start block generator main loop
	go s.mainLoop()
}

// mainLoop is main loop of service
func (s *Service) mainLoop() {
	log.Warn("[CoreService] Start main loop.")

	var start time.Time
	var end time.Duration

	for {
		select {
		case <-s.stop:
			return
		case <-s.ticker.C():
			log.Warnf("Slot: %d. Epoch: %d.", s.ticker.Slot(), s.ticker.Epoch())

			// Master node has key
			if s.proposer.Key() == nil {
				continue
			}

			start = time.Now()

			// generate block with block miner
			block, err := s.miner.ForgeBlock()
			if err != nil {
				log.Errorf("[CoreService] Error forging block: %s", err.Error())

				s.mu.Lock()
				s.statusErr = err
				s.mu.Unlock()
				return
			}

			// push block to events
			go s.SendBlock(block, true)

			if s.fullStatFlag {
				end = time.Since(start)
				log.Infof("[CoreService] Create block in: %s", common.StatFmt(end))
			}
		case bEvent := <-s.blockEvent:
			start = time.Now()

			block := bEvent.data

			// validate, save block and update SQL
			err := s.miner.FinalizeBlock(block, bEvent.isLocal)
			if err != nil {
				log.Errorf("[CoreService] Error finalizing block: %s", err.Error())

				s.mu.Lock()
				s.statusErr = err
				s.mu.Unlock()
				return
			}

			if s.fullStatFlag {
				end = time.Since(start)
				log.Infof("[CoreService] Finalize block in %s.", common.StatFmt(end))
			}

			blockSize := block.SizeSSZ() / 1024
			log.Warnf("[CoreService] Block #%d generated. Transactions in block: %d. Size: %d kB", block.Num, len(block.Transactions), blockSize)
		}
	}
}

// Stop stops tx generator service
func (s *Service) Stop() error {
	log.Warn("Stop Core Service.")

	close(s.stop)   // close stop chan
	s.cancelFunc()  // finish context

	return nil
}

func (s *Service) Status() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.statusErr
}

func (s *Service) SendBlock(block *prototype.Block, isLocal bool){
	s.blockFeed.Send(blockEvent{
		data: block,
		isLocal: isLocal,
	})
}

func (s *Service) waitInitialized(){
	for {
		select{
		case <-s.stop:
			return
		case st := <-s.stateEvent:
			if st == state.LocalDatabaseReady {
				return
			}
		}
	}
}

