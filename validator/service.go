package validator

import (
	"bytes"
	"context"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/attestation"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/backend/pos"
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
	stypes "github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/validator/types"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var log = logrus.WithField("prefix", "validator")
var _ shared.Service = (*Service)(nil)

const (
	votingDuration    = 3 * time.Second
	seedTimer         = 2 * time.Second
	attestationsCount = 50
	seedCount         = 50
	proposeCount      = 50
	seedPrime         = 4294967291 // prime number closest to 2**32-1
)

type Config struct {
	BlockFinalizer  consensus.BlockFinalizer
	AttestationPool consensus.AttestationPool
	ProposeFeed     events.Feed
	AttestationFeed events.Feed
	BlockFeed       events.Feed
	StateFeed       events.Feed
	SeedFeed        events.Feed
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
	seedEvent        chan *prototype.Seed

	proposeFeed     events.Feed
	attestationFeed events.Feed
	seedFeed        events.Feed
	blockFeed       events.Feed
	stateFeed       events.Feed

	// consensus Engine
	backend consensus.PoS

	unsubscribe func()

	// validation loop
	proposedBlock    *prototype.Block
	receivedBlock    *prototype.Block
	votingIsFinished bool
	blockVoting      map[string]*voting
	seedMap          map[string]*prototype.Seed
	finishVoting     <-chan time.Time
	waitSeed         <-chan time.Time

	// seed
	seed int64
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
			// generate a seed, send it.
			s.generateSeed()
		case seed := <-s.seedEvent:
			// Add to the map
			s.handleSeedEvent(seed)
		case <-s.waitSeed:
			s.mu.Lock()
			s.seedMap = make(map[string]*prototype.Seed)
			s.mu.Unlock()

			s.forgeBlock()
		case block := <-s.proposeEvent:
			s.mu.Lock()
			if s.receivedBlock != nil && bytes.Equal(block.Hash, s.receivedBlock.Hash) {
				s.mu.Unlock()
				continue
			}
			s.mu.Unlock()
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
			s.mu.Lock()

			s.votingIsFinished = true

			// gossip to network
			if s.proposedBlock == nil {
				continue
			}
			s.blockFeed.Send(s.proposedBlock)

			delete(s.blockVoting, common.Encode(s.proposedBlock.Hash))
			s.proposedBlock = nil
			s.mu.Unlock()
		case <-clearInterval.C:
			clearVoting()
		}
	}
}

func (s *Service) subscribeEvents() {
	proposeSub := s.proposeFeed.Subscribe(s.proposeEvent)
	attSub := s.attestationFeed.Subscribe(s.attestationEvent)
	seedSub := s.seedFeed.Subscribe(s.seedEvent)

	s.unsubscribe = func() {
		proposeSub.Unsubscribe()
		attSub.Unsubscribe()
		seedSub.Unsubscribe()
	}
}

func (s *Service) ProposeFeed() events.Feed {
	return s.proposeFeed
}

func (s *Service) AttestationFeed() events.Feed {
	return s.attestationFeed
}

func (s *Service) waitInitEvent() {
	stateEvent := make(chan state.State, 1)
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

	s.mu.Lock()
	if !s.backend.IsLeader(blockProposer, s.seed) {
		// todo mark validator as malicious
		log.Warnf("Not ordered block proposition from %s", blockProposer.Hex())
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	attestationType := types.Approve
	_, err := s.att.Validator().ValidateBlock(block, false)
	if err == attestation.ErrPreviousBlockNotExists {
		return nil
	} else if err != nil {
		log.Errorf("Failed block attestation: %s", err)
		attestationType = types.Reject
	}

	att, err := types.NewAttestation(block, s.proposer, attestationType)
	if err != nil {
		return errors.Wrap(err, "Attestation error")
	}

	if attestationType == types.Approve {
		s.mu.Lock()
		s.receivedBlock = block
		s.mu.Unlock()
	}

	s.attestationFeed.Send(att)

	return nil
}

func (s *Service) processAttestation(att *types.Attestation) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

	s.mu.Lock()
	// Check node is leader now
	log.Debugf("%s is leader: %b", s.proposer.Addr().Hex(), s.backend.IsLeader(s.proposer.Addr(), s.seed))
	if !s.backend.IsLeader(s.proposer.Addr(), s.seed) {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

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

// Generate the seed and send it to all peers
func (s *Service) generateSeed() {
	seed, err := stypes.NewSeed(s.proposer)
	if err != nil {
		log.Errorf("[ValidatorService] Error generating seed: %s", err.Error())

		s.mu.Lock()
		s.statusErr = err
		s.mu.Unlock()

		return
	}

	log.Debugf("Successfully generated seed: %d", seed.Seed)

	// Push the generated seed to events
	s.seedFeed.Send(seed)
	s.waitSeed = time.After(seedTimer)
}

// reseive the seed & calculate the resulting seed
func (s *Service) handleSeedEvent(seed *prototype.Seed) {
	// Checking for signature
	if err := stypes.GetSeedSigner().Verify(seed); err != nil {
		return
	}
	log.Debugf("Accepted incoming seed %d from %s", seed.Seed, common.BytesToAddress(seed.Proposer.Address).Hex())

	s.mu.Lock()
	address := common.BytesToAddress(seed.Proposer.Address).Hex()
	s.seedMap[address] = seed

	var result uint64 = 1
	for _, seed := range s.seedMap {
		se := uint64(seed.Seed)
		result = (result * se) % seedPrime
	}

	s.seed = int64(result)
	s.mu.Unlock()
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

	posConfig := pos.Config{
		AttestationPool: &cfg.AttestationPool,
	}
	engine := pos.New(posConfig)
	if !engine.IsValidator(proposer.Addr()) {
		return nil, errors.New("Attach validator key to start validator flow")
	}

	netCfg := params.RaidoConfig()
	forgerCfg := &forger.Config{
		EnableMetrics: cliCtx.Bool(flags.EnableMetrics.Name),
		BlockSize:     netCfg.BlockSize,
		Proposer:      proposer,
		Engine:        engine,
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
		proposeEvent:     make(chan *prototype.Block, proposeCount),
		attestationEvent: make(chan *types.Attestation, attestationsCount),
		seedEvent:        make(chan *prototype.Seed, seedCount),

		// feeds
		proposeFeed:     cfg.ProposeFeed,
		attestationFeed: cfg.AttestationFeed,
		blockFeed:       cfg.BlockFeed,
		stateFeed:       cfg.StateFeed,
		seedFeed:        cfg.SeedFeed,

		// engine
		backend: engine,

		// validator loop
		blockVoting: map[string]*voting{},

		seedMap: map[string]*prototype.Seed{},
	}

	return srv, nil
}
