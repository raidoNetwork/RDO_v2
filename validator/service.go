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

const (
	votingDuration    = 3 * time.Second
	attestationsCount = 10
)

type Config struct {
	BlockFinalizer  consensus.BlockFinalizer
	AttestationPool consensus.AttestationPool
	ProposeFeed     events.Feed
	AttestationFeed events.Feed
	BlockFeed       events.Feed
	StateFeed       events.Feed
	Context         context.Context
}

type Service struct {
	cliCtx     *cli.Context
	ctx        context.Context
	cancelFunc context.CancelFunc
	statusErr  error
	bc         consensus.BlockFinalizer
	att        consensus.AttestationPool
	proposer   *keystore.ValidatorAccount

	miner  *forger.Forger // block miner
	ticker *slot.SlotTicker

	mu sync.Mutex

	// events
	proposeEvent     chan *prototype.Block
	attestationEvent chan *types.Attestation

	proposeFeed     events.Feed
	attestationFeed events.Feed
	blockFeed       events.Feed
	stateFeed       events.Feed

	// consensus Engine
	backend consensus.PoA

	unsubscribe func()

	// validation loop
	proposedBlock    *prototype.Block
	votingIsFinished bool
	blockVoting      map[string]*voting
	finishVoting     <-chan time.Time
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
	log.Info("Stop validator service")

	s.cancelFunc()
	s.unsubscribe()
	s.ticker.Stop()

	return nil
}

func (s *Service) loop() {
	clearInterval := time.NewTicker(30 * time.Second)
	clearVoting := func() {
		proposedBlockHash := ""
		if s.proposedBlock != nil {
			proposedBlockHash = common.Encode(s.proposedBlock.Hash)
		}

		for hash := range s.blockVoting {
			if hash == proposedBlockHash {
				continue
			}

			delete(s.blockVoting, hash)
		}
	}

	defer clearInterval.Stop()

	for {
		select {
		case <-s.ticker.C():
			s.forgeBlock()
		case block := <-s.proposeEvent:
			err := s.verifyBlock(block)
			if err != nil {
				panic(err)
			}
		case att := <-s.attestationEvent:
			s.processAttestation(att)
		case <-s.ctx.Done():
			log.Warn("Finish validation process...")
			return
		case <-s.finishVoting:
			s.votingIsFinished = true

			// gossip to network
			s.blockFeed.Send(s.proposedBlock)

			delete(s.blockVoting, common.Encode(s.proposedBlock.Hash))
			s.proposedBlock = nil
		case <-clearInterval.C:
			clearVoting()
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
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case st := <-stateEvent:
			if st == state.Synced {
				err := s.ticker.StartFromTimestamp(s.bc.GetGenesis().Timestamp)
				if err != nil {
					panic("Zero Genesis time")
				}
				return
			}
		}
	}
}

func (s *Service) verifyBlock(block *prototype.Block) error {
	blockProposer := common.BytesToAddress(block.Proposer.Address)

	if blockProposer.Hex() == s.proposer.Addr().Hex() {
		log.Debug("Skip local block attestation")
		return nil
	}

	if !s.backend.IsLeader(blockProposer) {
		// todo mark validator as malicious
		log.Warnf("Not ordered block proposition from %s", blockProposer.Hex())
		return nil
	}

	attestationType := types.Approve
	_, err := s.att.Validator().ValidateBlock(block, s.att.TxPool(), false)
	if err != nil {
		log.Errorf("Failed block attestation: %s", err)
		attestationType = types.Reject
	}

	att, err := types.NewAttestation(block, s.proposer, attestationType)
	if err != nil {
		return errors.Wrap(err, "Attestation error")
	}

	s.attestationFeed.Send(att)

	return nil
}

func (s *Service) processAttestation(att *types.Attestation) {
	proposer := common.BytesToAddress(att.Block.Proposer.Address)
	if !s.backend.IsValidator(proposer) {
		return
	}

	blockHash := common.Encode(att.Block.Hash)
	if att.Validator.Hex() != s.proposer.Addr().Hex() {
		log.Debugf(
			"Receive attestation for block #%d %s",
			att.Block.Num,
			blockHash,
		)

		if err := types.VerifyAttestationSign(att); err != nil {
			// todo mark validator as malicious
			log.Warnf(
				"Malicious validator %s. Wrong sign on block %d %s.",
				common.Encode(att.Signature.Address),
				att.Block.Num,
				blockHash,
			)
			return
		}
	}

	if _, exists := s.blockVoting[blockHash]; !exists {
		s.blockVoting[blockHash] = &voting{
			started: time.Now(),
		}
	}

	isLocalProposer := proposer.Hex() == s.proposer.Addr().Hex()
	canUpdateProposedBlock := isLocalProposer && !s.votingIsFinished
	if att.Type == types.Approve {
		s.blockVoting[blockHash].approved += 1

		if canUpdateProposedBlock {
			s.proposedBlock.Approvers = append(s.proposedBlock.Approvers, att.Signature)
		}
	} else {
		s.blockVoting[blockHash].rejected += 1

		if canUpdateProposedBlock {
			s.proposedBlock.Slashers = append(s.proposedBlock.Slashers, att.Signature)
		}
	}
}

func (s *Service) forgeBlock() {
	// Check node is leader now
	if !s.backend.IsLeader(s.proposer.Addr()) {
		return
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
	s.proposedBlock = block

	log.Warnf("Propose block #%d %s", block.Num, common.Encode(block.Hash))

	// push block to events
	s.proposeFeed.Send(block)

	s.finishVoting = time.After(votingDuration)
	s.votingIsFinished = false

	log.Debugf("Block #%d forged in %d ms", block.Num, time.Since(start).Milliseconds())
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

	log.Infof("Register validator key %s", proposer.Addr().Hex())

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

		bc:  cfg.BlockFinalizer,
		att: cfg.AttestationPool,

		ticker: slot.NewSlotTicker(),

		// events
		proposeEvent:     make(chan *prototype.Block, 1),
		attestationEvent: make(chan *types.Attestation, attestationsCount),

		// feeds
		proposeFeed:     cfg.ProposeFeed,
		attestationFeed: cfg.AttestationFeed,
		blockFeed:       cfg.BlockFeed,
		stateFeed:       cfg.StateFeed,

		// engine
		backend: engine,

		// validator loop
		blockVoting: map[string]*voting{},
	}

	return srv, nil
}