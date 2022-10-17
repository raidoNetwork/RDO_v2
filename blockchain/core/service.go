package core

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared"
	utypes "github.com/raidoNetwork/RDO_v2/utils/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "core")
var _ shared.Service = (*Service)(nil)

type Config struct{
	BlockFinalizer  consensus.BlockFinalizer
	AttestationPool consensus.AttestationPool
	StateFeed events.Feed
	BlockFeed events.Feed
	Context context.Context
}

// NewService creates new CoreService
func NewService(cliCtx *cli.Context, cfg *Config) (*Service, error) {
	ctx, finish := context.WithCancel(cfg.Context)

	srv := &Service{
		cliCtx:     cliCtx,
		ctx:        ctx,
		cancelFunc: finish,
		att:	    cfg.AttestationPool,

		ticker:     slot.Ticker(),

		// events
		blockEvent: make(chan *prototype.Block, 5),
		stateEvent: make(chan state.State, 1),

		// feeds
		blockFeed: cfg.BlockFeed,
		stateFeed: cfg.StateFeed,

		bc: cfg.BlockFinalizer,
	}

	return srv, nil
}

// Service implements blockchain service for blockchain update, read and creating new blocks.
type Service struct {
	cliCtx       *cli.Context
	ctx          context.Context
	cancelFunc   context.CancelFunc
	statusErr    error
	att			 consensus.AttestationPool
	bc			 consensus.BlockFinalizer

	ticker *slot.SlotTicker

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

	// start block generator main loop
	go s.mainLoop()
}

// mainLoop is main loop of service
func (s *Service) mainLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.ticker.C():
			updateCoreMetrics()
		case block := <-s.blockEvent:
			start := time.Now()

			err := s.FinalizeBlock(block)
			if err != nil {
				log.Errorf("[CoreService] Error finalizing block: %s", err.Error())

				if !errors.Is(err, consensus.ErrKnownBlock) {
					s.mu.Lock()
					s.statusErr = err
					s.mu.Unlock()

					continue
				}
			}

			blockSize := block.SizeSSZ() / 1024
			log.Warnf("[CoreService] Block #%d finalized. Transactions in block: %d. Size: %d kB.", block.Num, len(block.Transactions), blockSize)
			log.Debugf("Block #%d finalized time %d ms", block.Num, time.Since(start).Milliseconds())
		}
	}
}

// Stop stops tx generator service
func (s *Service) Stop() error {
	log.Info("Stop Core service")
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
		case <-s.ctx.Done():
			return
		case st := <-s.stateEvent:
			switch st {
			case state.LocalSynced:
				// start slot ticker
				err := s.ticker.StartFromTimestamp(s.bc.GetGenesis().Timestamp)
				if err != nil {
					panic("Zero Genesis time")
				}
			case state.Synced:
				return
			}
		}
	}
}

func (s *Service) subscribeOnEvents() {
	s.stateFeed.Subscribe(s.stateEvent)
	s.blockFeed.Subscribe(s.blockEvent)
}

func (s *Service) FinalizeBlock(block *prototype.Block) error {
	log.Debugf("Finalizing block #%d", block.Num)
	start := time.Now()

	if block.Num == 0 {
		return s.att.Validator().ValidateGenesis(block)
	}

	// validate block
	failedTx, err := s.att.Validator().ValidateBlock(block, s.att.TxPool(), true)
	if err != nil {
		if failedTx != nil {
			s.att.TxPool().Finalize(failedTx)
		}
		return errors.Wrap(err, "ValidateBlockError")
	}

	// save block
	err = s.bc.FinalizeBlock(block)
	if err != nil {
		return errors.Wrap(err, "FinalizeBlockError")
	}

	typedBatch := utypes.PbTxBatchToTyped(block.Transactions)

	// clear pool
	s.att.TxPool().Finalize(typedBatch)

	// update stake pool data
	err = s.att.StakePool().FinalizeStaking(typedBatch)
	if err != nil {
		return errors.Wrap(err, "StakePool error")
	}

	err = s.bc.CheckBalance()
	if err != nil {
		return errors.Wrap(err, "Balances inconsistency")
	}

	finalizeBlockTime.Observe(float64(time.Since(start).Milliseconds()))
	return nil
}

