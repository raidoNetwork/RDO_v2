package core

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/miner"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/keystore"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "core")

// NewService creates new CoreService
func NewService(cliCtx *cli.Context, bc consensus.BlockForger, att consensus.AttestationPool) (*Service, error) {
	statFlag := cliCtx.Bool(flags.SrvStat.Name)
	debugStatFlag := cliCtx.Bool(flags.SrvDebugStat.Name)
	dataDir := cliCtx.String(cmd.DataDirFlag.Name)

	cfg := params.RaidoConfig()

	proposer, err := keystore.NewValidatorAccountFromFile(dataDir)
	if err != nil {
		return nil, err
	}

	minerCfg := &miner.MinerConfig{
		ShowStat: statFlag,
		ShowFullStat: debugStatFlag,
		BlockSize: cfg.BlockSize,
		Proposer: proposer,
	}

	// new block miner
	forger := miner.NewMiner(bc, att, minerCfg)

	ctx, finish := context.WithCancel(context.Background())

	srv := &Service{
		cliCtx:     cliCtx,
		ctx:        ctx,
		cancelFunc: finish,
		miner:      forger,

		stop: make(chan struct{}),
		ticker: slot.Ticker(),

		// flags
		fullStatFlag: statFlag,
		expStatFlag:  debugStatFlag,
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

	miner *miner.Miner            // block miner
	ticker *slot.SlotTicker

	// flags
	fullStatFlag bool
	expStatFlag  bool

	mu sync.Mutex
}

// Start service work
func (s *Service) Start() {
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

			start = time.Now()

			block, err := s.miner.ForgeBlock()
			if err != nil {
				log.Errorf("[ChainService] mainLoop: Error: %s", err.Error())

				s.mu.Lock()
				s.statusErr = err
				s.mu.Unlock()
				return
			}

			if s.fullStatFlag {
				end = time.Since(start)
				log.Infof("[ChainService] Create block in: %s", common.StatFmt(end))
			}

			log.Warnf("[CoreService] Block #%d generated. Transactions in block: %d.", block.Num, len(block.Transactions))
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

