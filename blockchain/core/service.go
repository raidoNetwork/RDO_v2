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

type Config struct{
	BlockForger consensus.BlockForger
	AttestationPool consensus.AttestationPool
	StateFeed events.Feed
	BlockFeed events.Feed
}

// NewService creates new CoreService
func NewService(cliCtx *cli.Context, cfg *Config) (*Service, error) {
	statFlag := cliCtx.Bool(flags.SrvStat.Name)
	debugStatFlag := cliCtx.Bool(flags.DebugLogging.Name)
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
	} else {
		msg = "Slave node"
	}
	log.Info(msg)

	minerCfg := &miner.Config{
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
		blockEvent: make(chan *prototype.Block, 1),
		stateEvent: make(chan state.State),

		// feeds
		blockFeed: cfg.BlockFeed,
		stateFeed: cfg.StateFeed,
	}

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
	blockEvent chan *prototype.Block

	blockFeed events.Feed
	stateFeed events.Feed
}

// Start service work
func (s *Service) Start() {
	s.subscribeOnEvents()
	s.waitInitialized()

	// start slot ticker
	genesisTime := time.Unix(0, int64(s.bc.GetGenesis().Timestamp))
	err := slot.Ticker().Start(genesisTime)
	if err != nil {
		panic("Zero Genesis time")
	}

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

				s.stateFeed.Send(state.ForgeFailed)
				return
			}

			// push block to events
			s.blockFeed.Send(block)

			if s.fullStatFlag {
				end = time.Since(start)
				log.Infof("[CoreService] Create block in: %s", common.StatFmt(end))
			}
		case block := <-s.blockEvent:
			start = time.Now()

			// validate, save block and update SQL
			err := s.miner.FinalizeBlock(block)
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

func (s *Service) waitInitialized() {
	for {
		select{
		case <-s.stop:
			return
		case st := <-s.stateEvent:
			if st == state.Synced {
				return
			}
		}
	}
}

func (s *Service) subscribeOnEvents() {
	s.stateFeed.Subscribe(s.stateEvent)
	s.blockFeed.Subscribe(s.blockEvent)
}

