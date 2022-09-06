package validator

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/backend/poa"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/forger"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	vflags "github.com/raidoNetwork/RDO_v2/cmd/validator/flags"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/validator/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"path/filepath"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "validator")
var _ shared.Service = (*Service)(nil)

const votingDuration = 5 * time.Second

type Config struct{
	BlockFinalizer  consensus.BlockFinalizer
	AttestationPool consensus.AttestationPool
	ProposeFeed events.Feed
	AttestationFeed events.Feed
	BlockFeed	events.Feed
	StateFeed events.Feed
	Context context.Context
}

type Service struct {
	cliCtx       *cli.Context
	ctx          context.Context
	cancelFunc   context.CancelFunc
	statusErr    error
	bc			 consensus.BlockFinalizer
	att			 consensus.AttestationPool
	proposer 	 *keystore.ValidatorAccount

	miner *forger.Forger // block miner
	ticker *slot.SlotTicker

	mu sync.Mutex

	// events
	proposeEvent chan *prototype.Block
	attestationEvent chan *types.Attestation

	proposeFeed events.Feed
	attestationFeed events.Feed
	blockFeed events.Feed
	stateFeed events.Feed

	// consensus Engine
	backend consensus.PoA

	unsubscribe func()
}

func (s *Service) Start() {
	s.subscribeEvents()
	s.waitInitEvent()

	log.Warnf("Start validator node %s", s.proposer.Addr().Hex())

	go s.loop()
}

func (s *Service) Status() error {
	return nil
}

func (s *Service) Stop() error {
	s.unsubscribe()
	s.cancelFunc()

	return nil
}

func (s *Service) loop() {
	var finishVoting <-chan time.Time
	var proposedBlock *prototype.Block
	blockVoting := map[string]*voting{}
	votingIsFinished := false

	for {
		select {
		case <-s.ticker.C():
			// Check node is leader now
			if !s.backend.IsLeader(s.proposer.Addr()) {
				continue
			}

			start := time.Now()

			// generate block with block miner
			block, err := s.miner.ForgeBlock()
			if err != nil {
				log.Errorf("[ValidatorService] Error forging block: %s", err.Error())

				s.mu.Lock()
				s.statusErr = err
				s.mu.Unlock()

				return
			}

			// setup block for updates
			proposedBlock = block

			log.Warnf("Propose block #%d %s", block.Num, common.Encode(block.Hash))

			// push block to events
			s.proposeFeed.Send(block)
			finishVoting = time.After(votingDuration)
			votingIsFinished = false

			log.Debugf("Block #%d forged in %d ms", block.Num, time.Since(start).Milliseconds())
		case block := <-s.proposeEvent:
			if common.Encode(block.Proposer.Address) == s.proposer.Addr().Hex() {
				continue
			}

			attestationType := types.Approve
			_, err := s.att.Validator().ValidateBlock(block, s.att.TxPool())
			if err != nil {
				log.Errorf("Failed block attestation: %s", err)
				attestationType = types.Reject
			}

			att, err := types.NewAttestation(block, s.proposer, attestationType)
			if err != nil {
				log.Errorf("Attestation error: %s", err)
				return
			}

			s.attestationFeed.Send(att)
		case att := <-s.attestationEvent:
			if !s.backend.IsValidator(common.BytesToAddress(att.Block.Proposer.Address)) {
				continue
			}

			blockHash := common.Encode(att.Block.Hash)
			err := types.VerifyAttestationSign(att)
			if err != nil {
				log.Warnf(
					"Malicious validator %s. Wrong sign on block %d %s.",
					common.Encode(att.Signature.Address),
					att.Block.Num,
					blockHash,
				)
				continue
			}

			if _, exists := blockVoting[blockHash]; !exists {
				blockVoting[blockHash] = &voting{}
			}

			isLocalProposer := common.Encode(att.Block.Proposer.Address) == s.proposer.Addr().Hex()
			if att.Type == types.Approve {
				blockVoting[blockHash].approved += 1

				if isLocalProposer && !votingIsFinished {
					proposedBlock.Approvers = append(proposedBlock.Approvers, att.Signature)
				}
			} else {
				blockVoting[blockHash].rejected += 1

				if isLocalProposer && !votingIsFinished {
					proposedBlock.Slashers = append(proposedBlock.Slashers, att.Signature)
				}
			}
		case <-s.ctx.Done():
			return
		case <-finishVoting:
			votingIsFinished = true

			// gossip to network
			s.blockFeed.Send(proposedBlock)

			delete(blockVoting, common.Encode(proposedBlock.Hash))
			proposedBlock = nil
		}
	}
}

func (s *Service) subscribeEvents() {
	proposeSub := s.proposeFeed.Subscribe(s.proposeEvent)
	attSub := s.attestationFeed.Subscribe(s.attestationEvent)

	s.unsubscribe = func() {
		proposeSub.Unsubscribe()
		attSub.Unsubscribe()
	}
}

func (s *Service) ProposeFeed() events.Feed {
	return s.proposeFeed
}

func (s *Service) AttestationFeed() events.Feed {
	return s.attestationFeed
}

func (s *Service) waitInitEvent() {
	stateEvent := make(chan state.State)
	subs := s.stateFeed.Subscribe(stateEvent)

	defer func() {
		subs.Unsubscribe()
	} ()

	for {
		select{
		case <-s.ctx.Done():
			return
		case st := <-stateEvent:
			switch st {
			case state.Synced:
				err := s.ticker.StartFromTimestamp(s.bc.GetGenesis().Timestamp)
				if err != nil {
					panic("Zero Genesis time")
				}
				return
			}
		}
	}
}

func New(cliCtx *cli.Context, cfg *Config) (*Service, error) {
	var path string
	validatorPath := cliCtx.String(vflags.ValidatorKey.Name)
	if validatorPath != "" {
		path = validatorPath
	} else {
		dataDir := cliCtx.String(cmd.DataDirFlag.Name)
		path = filepath.Join(dataDir, "validator", "validator.key")
	}

	proposer, err := keystore.NewValidatorAccountFromFile(path)
	if err != nil {
		return nil, err
	}

	if proposer.Key() == nil {
		return nil, errors.New("Not found validator key file")
	}

	engine := poa.New()
	if !engine.IsValidator(proposer.Addr()) {
		return nil, errors.New("Attach validator key to start validator flow")
	}

	netCfg := params.RaidoConfig()
	forgerCfg := &forger.Config{
		EnableMetrics: cliCtx.Bool(flags.EnableMetrics.Name),
		BlockSize:     netCfg.BlockSize,
		Proposer:      proposer,
	}

	// new block miner
	blockForger := forger.New(cfg.BlockFinalizer, cfg.AttestationPool, forgerCfg)

	ctx, finish := context.WithCancel(cfg.Context)
	srv := &Service{
		cliCtx:     cliCtx,
		ctx:        ctx,
		cancelFunc: finish,
		miner:      blockForger,
		proposer:   proposer,

		bc:         cfg.BlockFinalizer,
		att:	    cfg.AttestationPool,

		ticker:     slot.NewSlotTicker(),

		// events
		proposeEvent: make(chan *prototype.Block, 1),
		attestationEvent: make(chan *types.Attestation, 1),

		// feeds
		proposeFeed: cfg.ProposeFeed,
		attestationFeed: cfg.AttestationFeed,
		blockFeed: cfg.BlockFeed,
		stateFeed: cfg.StateFeed,

		// engine
		backend: engine,
	}

	return srv, nil
}