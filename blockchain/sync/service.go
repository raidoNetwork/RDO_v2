package sync

import (
	"context"
	"runtime/debug"
	"sync/atomic"
	"time"

	ssz "github.com/ferranbt/fastssz"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "syncBlocks")
var _ shared.Service = (*Service)(nil)
var MainService *Service

const (
	txGossipCount    = 400
	blockGossipCount = 200
	stateCount       = 1

	ttfbTimeout = 5 * time.Second

	blocksPerRequest = 64
)

var (
	ErrAlreadySynced = errors.New("The node is already synced with network")
)

type ValidatorCfg struct {
	ProposeFeed     *events.Feed
	AttestationFeed *events.Feed
	SeedFeed        *events.Feed
	Enabled         bool
}

type Config struct {
	BlockFeed    *events.Feed
	TxFeed       *events.Feed
	StateFeed    *events.Feed
	Blockchain   BlockchainInfo
	Storage      BlockStorage
	P2P          P2P
	DisableSync  bool
	MinSyncPeers int
	Validator    ValidatorCfg
}

func NewService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	srv := &Service{
		cfg:               cfg,
		txEvent:           make(chan *types.Transaction, txGossipCount),
		blockEvent:        make(chan *prototype.Block, blockGossipCount),
		stateEvent:        make(chan state.State, stateCount),
		notificationBlock: make(chan p2p.Notty, blockGossipCount),
		notificationTx:    make(chan p2p.Notty, txGossipCount),
		ctx:               ctx,
		cancel:            cancel,
		initialized:       make(chan struct{}),
		stakeSynced:       make(chan struct{}),
		connected:         make(chan struct{}),
		syncLock:          make(chan struct{}, 1),
		synced:            0,
	}

	// subscribe on new events
	srv.subscribeEvents()

	MainService = srv

	return srv
}

type Service struct {
	cfg *Config

	txEvent           chan *types.Transaction
	blockEvent        chan *prototype.Block
	stateEvent        chan state.State
	notificationBlock chan p2p.Notty
	notificationTx    chan p2p.Notty

	ctx    context.Context
	cancel context.CancelFunc

	initialized chan struct{}
	stakeSynced chan struct{}
	connected   chan struct{}
	syncLock    chan struct{}

	synced int32

	forkBlockEvent chan *prototype.Block
	bq             *blockQueue
}

func (s *Service) Start() {
	go s.stateListener()
	go s.cfg.P2P.AddConnectionHandlers(func(ctx context.Context, id peer.ID) error {
		return s.metaRequest(ctx, id)
	}, func(_ context.Context, _ peer.ID) error {
		return nil
	})

	// set stream handler for block receiving
	s.addStreamHandler(p2p.MetaProtocol, s.metaHandler)
	s.addStreamHandler(p2p.BlockRangeProtocol, s.blockRangeHandler)

	s.pushStateEvent(state.ConnectionHandlersReady)

	// wait for local database synced
	<-s.initialized

	// Wait for the stake pool to sync
	<-s.stakeSynced

	log.Info("Block on connected state")
	<-s.connected

	s.metaReq()

	// start block queue watcher
	go s.forkWatcher()

	// listen incoming data
	go s.listenIncoming()

	log.Info("Start blockchain synced...")

	// sync state with network
	s.syncLock <- struct{}{}
	if !s.cfg.DisableSync && !slot.Ticker().GenesisAfter() {
		err := s.SyncWithNetwork()
		if err != nil && err != ErrAlreadySynced {
			panic(err)
		}
	}
	atomic.AddInt32(&s.synced, 1)

	log.Warnf("Node synced with network")
	s.pushSyncedState()

	// gossip new blocks and transactions
	go s.gossipEvents()
	go s.maintainSync()

	if s.cfg.Validator.Enabled {
		go s.listenValidatorTopics()
		go s.gossipValidatorMessages()
	}
}

func (s *Service) gossipEvents() {
	for {
		select {
		case block := <-s.blockEvent:
			raw, err := serialize.MarshalBlock(block)
			if err != nil {
				log.Errorf("Error marshaling block: %s", err)
				continue
			}

			err = s.cfg.P2P.Publish(p2p.BlockTopic, raw)
			if err != nil {
				log.Errorf("Error sending block: %s", err)
			}
		case td := <-s.txEvent:
			raw, err := serialize.MarshalTx(td.GetTx())
			if err != nil {
				log.Errorf("Error marshaling transaction: %s", err)
				continue
			}

			err = s.cfg.P2P.Publish(p2p.TxTopic, raw)
			if err != nil {
				log.Errorf("Error sending transaction: %s", err)
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Service) Stop() error {
	log.Info("Stop sync service")

	// cancel context
	s.cancel()

	// todo update logic

	return nil
}

func (s *Service) Status() error {
	return nil
}

func (s *Service) stateListener() {
	sub := s.cfg.StateFeed.Subscribe(s.stateEvent)
	defer sub.Unsubscribe()

	for {
		select {
		case <-s.ctx.Done():
			return
		case st := <-s.stateEvent:
			switch st {
			case state.Initialized:
				// do nothing
			case state.StakePoolInitialized:
				close(s.stakeSynced)
			case state.Connected:
				close(s.connected)
			case state.LocalSynced:
				close(s.initialized)
			case state.ConnectionHandlersReady:
				// do nothing
			case state.Synced:
				// do nothing
			default:
				log.Errorf("Unknown state event %d", st)
				return
			}
		}
	}
}

func (s *Service) listenIncoming() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case notty := <-s.notificationBlock:
			block, err := serialize.UnmarshalBlock(notty.Data)
			if err != nil {
				log.Errorf("Error unmarshaling block: %s", err)
				break
			}

			if atomic.LoadInt32(&s.synced) == 0 {
				s.forkBlockEvent <- block
			} else {
				bQueue := s.freeQueue()
				for b := range bQueue {
					s.cfg.BlockFeed.Send(b)
				}

				s.cfg.BlockFeed.Send(block)
			}

			receivedMessages.WithLabelValues(p2p.BlockTopic).Inc()
		case notty := <-s.notificationTx:
			// skip tx processing while synscing
			if atomic.LoadInt32(&s.synced) == 0 {
				continue
			}

			tx, err := serialize.UnmarshalTx(notty.Data)
			if err != nil {
				log.Errorf("Error unmarshaling transaction: %s", err)
				break
			}

			s.cfg.TxFeed.Send(types.NewTransaction(tx))
			receivedMessages.WithLabelValues(p2p.TxTopic).Inc()
		}
	}
}

func (s *Service) pushSyncedState() {
	s.pushStateEvent(state.Synced)
}

func (s *Service) pushStateEvent(st state.State) {
	s.cfg.StateFeed.Send(st)
}

// subscribeEvents on updates
func (s *Service) subscribeEvents() {
	s.cfg.P2P.NotifierTx().Subscribe(s.notificationTx)
	s.cfg.P2P.NotifierBlock().Subscribe(s.notificationBlock)
	s.cfg.TxFeed.Subscribe(s.txEvent)
	s.cfg.BlockFeed.Subscribe(s.blockEvent)
}

func (s *Service) addStreamHandler(topic string, handle streamHandler) {
	s.cfg.P2P.SetStreamHandler(topic, func(stream network.Stream) {
		defer func() {
			if r := recover(); r != nil {
				log.WithField("error", r).Error("Panic occurred")
				log.Errorf("%s", debug.Stack())
			}
		}()
		ctx, cancel := context.WithTimeout(s.ctx, ttfbTimeout)
		defer cancel()

		defer func() {
			_ = stream.Close()
			log.Debugf("Close stream on response for %s", string(stream.Protocol()))
		}()

		log := log.WithField("peer", stream.Conn().RemotePeer().Pretty()).WithField("topic", string(stream.Protocol()))

		if err := stream.SetReadDeadline(time.Now().Add(ttfbTimeout)); err != nil {
			log.WithError(err).Debug("Could not set stream read deadline")
			return
		}

		// Increment message received counter.
		receivedMessages.WithLabelValues(topic).Inc()

		var msg ssz.Unmarshaler
		switch topic {
		case p2p.MetaProtocol:
			msg = &prototype.Metadata{}
		case p2p.BlockRangeProtocol:
			msg = &prototype.BlockRequest{}
		default:
			log.Errorf("Undefined message topic %s", topic)
			return
		}

		if err := s.cfg.P2P.DecodeStream(stream, msg); err != nil {
			log.WithError(err).Debug("Could not decode stream message")
			return
		}

		if err := handle(ctx, msg, stream); err != nil {
			messageFailedProcessingCounter.WithLabelValues(topic).Inc()
			log.Errorf("Could not handle p2p RPC: %s", err)
		}
	})
}

func GetMainService() *Service {
	return MainService
}

func (s *Service) metaReq() {
	peers := s.cfg.P2P.PeerStore().Connected()
	for _, data := range peers {
		if time.Since(data.LastUpdate) < p2p.PeerMetaUpdateInterval {
			continue
		}

		if err := s.metaRequest(s.ctx, data.Id); err != nil {
			log.Error(err)
		}
	}
}

func (s *Service) maintainSync() {
	tick := time.NewTicker(p2p.PeerMetaUpdateInterval)
	for {
		select {
		case <-tick.C:
			s.metaReq()
			err := s.SyncWithNetwork()
			if err != nil && err != ErrAlreadySynced {
				log.Errorf("Error while maintaining sync: %s", err)
			}
		}
	}
}
