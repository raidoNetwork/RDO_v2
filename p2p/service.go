package p2p

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	ssz "github.com/ferranbt/fastssz"
	"github.com/libp2p/go-libp2p"
	phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/protocol/identify"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/utils/async"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "p2p")
var maxDialTimeout = time.Duration(params.RaidoConfig().ResponseTimeout) * time.Second

const (
	messageQueueSize = 256
	MaxChunkSize     = 1 << 20
	failPublishLimit = 10
)

type ConnectionHandler func(context.Context, peer.ID) error

type Config struct {
	Host                string
	Port                int
	BootstrapNodes      []string
	DataDir             string
	StateFeed           events.Feed
	EnableNAT           bool
	ListenValidatorData bool
}

func NewService(ctx context.Context, cfg *Config) (srv *Service, err error) {
	nodePrivKey, err := getNodeKey(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	p2pHostAddr := cfg.Host
	if p2pHostAddr == "" {
		p2pHostAddr, err = getIPaddr()
		if err != nil {
			return nil, err
		}
	} else {
		hostIp := net.ParseIP(p2pHostAddr)
		if hostIp.To4() == nil {
			return nil, errors.Errorf("Invalid host IP address given %s", p2pHostAddr)
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	srv = &Service{
		nodeKey:     nodePrivKey,
		ctx:         ctx,
		cancel:      cancel,
		cfg:         cfg,
		topics:      map[string]*pubsub.Topic{},
		subs:        map[string]*pubsub.Subscription{},
		stateEvent:  make(chan state.State, 1),
		initialized: make(chan struct{}),
	}

	opts, err := srv.optionsList(nodePrivKey, p2pHostAddr, cfg)
	if err != nil {
		return nil, err
	}

	netHost, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}

	srv.host = netHost
	srv.host.RemoveStreamHandler(identify.IDDelta)

	srv.id = netHost.ID()
	srv.peerStore = NewPeerStore()

	psOpts := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign),
		pubsub.WithNoAuthor(),
		pubsub.WithMessageIdFn(msgId),
		pubsub.WithPeerOutboundQueueSize(messageQueueSize),
		pubsub.WithValidateQueueSize(messageQueueSize),
		pubsub.WithGossipSubParams(pubsubGossipParam()),
		pubsub.WithPeerScore(peerScoringParams()),
		pubsub.WithSubscriptionFilter(srv),
	}

	gs, err := pubsub.NewGossipSub(ctx, netHost, psOpts...)
	if err != nil {
		return nil, err
	}

	srv.pubsub = gs
	return srv, nil
}

type Service struct {
	nodeKey    *nodeKey
	host       phost.Host
	id         peer.ID
	pubsub     *pubsub.PubSub
	topics     map[string]*pubsub.Topic
	subs       map[string]*pubsub.Subscription
	stateEvent chan state.State

	ctx       context.Context
	cancel    context.CancelFunc
	topicLock sync.Mutex

	cfg *Config

	startFail error

	notifier          events.Bus
	validatorNotifier events.Bus

	// discovery
	dht *dht.IpfsDHT

	peerStore *PeerStore

	initialized chan struct{}
}

func (s *Service) Start() {
	go s.stateListener()

	// wait local db syncing
	<-s.initialized

	s.logID()

	// start new peer search
	err := s.setupDiscovery()
	if err != nil {
		s.startFail = err
		return
	}

	// connect to bootstrap nodes
	s.connectPeers()

	async.WithInterval(s.ctx, 5*time.Second, func() {
		s.updateMetrics()
	})

	// wait full sync
	<-s.initialized

	// join topics
	err = s.SubscribeAll()
	if err != nil {
		log.Error(err)
		s.startFail = err
		return
	}

	// list new messages
	go s.readMessages()
}

func (s *Service) Stop() error {
	log.Info("P2P shutdown...")
	s.cancel()

	// cancel stream handlers
	for _, p := range s.host.Mux().Protocols() {
		s.host.RemoveStreamHandler(protocol.ID(p))
	}

	// cancel topic subscribes
	for _, t := range s.pubsub.GetTopics() {
		s.topicLock.Lock()
		if sub, exists := s.subs[t]; exists {
			sub.Cancel()
		}
		s.topicLock.Unlock()
	}

	if err := s.host.Close(); err != nil {
		return err
	}

	return nil
}

func (s *Service) Status() error {
	if s.startFail != nil {
		log.Errorf("Start error: %s", s.startFail)
		return s.startFail
	}

	return nil
}

func (s *Service) connectPeers() {
	infos := s.getPeerInfo(s.cfg.BootstrapNodes)
	if len(infos) == 0 {
		log.Error("There are no peers to connect.")
		return
	}

	for _, info := range infos {
		err := s.connectPeer(info)
		if err != nil {
			log.Error(err)
		}
	}
}

func (s *Service) getPeerInfo(addrs []string) []peer.AddrInfo {
	res := make([]peer.AddrInfo, 0, len(addrs))
	for _, addr := range addrs {
		peerInfo, err := peer.AddrInfoFromString(addr)
		if err != nil {
			log.WithError(err).Error("Error parsing peer info")
			continue
		}

		res = append(res, *peerInfo)
	}

	return res
}

func (s *Service) connectPeer(info peer.AddrInfo) error {
	ctx, cancel := context.WithTimeout(s.ctx, maxDialTimeout)
	defer cancel()

	if s.peerStore.IsBad(info.ID) {
		return errors.New("refuse connection to the bad peer")
	}

	if err := s.host.Connect(ctx, info); err != nil {
		log.Errorf("Error connection to the peer %v: %s", info.String(), err)
		return err
	}

	log.Infof("Connect to the %s", info.String())
	s.peerStore.Connect(info.ID)

	return nil
}

func (s *Service) readMessages() {
	for t := range topicMap {
		go s.listenTopic(t)
	}

	if s.cfg.ListenValidatorData {
		for t := range validatorMap {
			go s.listenTopic(t)
		}
	}
}

func (s *Service) listenTopic(topic string) {
	s.topicLock.Lock()
	sub, exists := s.subs[topic]
	id := s.id
	s.topicLock.Unlock()

	if !exists {
		return
	}

	for {
		msg, err := sub.Next(s.ctx)
		if err != nil {
			if err != context.Canceled {
				log.WithError(err).Error("error listening topic " + topic)
			}

			sub.Cancel()
			return
		}

		if msg.ReceivedFrom == id {
			continue
		}

		_, isValidatorTopic := validatorMap[topic]
		go s.receiveMessage(msg, isValidatorTopic)
	}
}

func (s *Service) SubscribeAll() error {
	for t := range topicMap {
		err := s.subscribeTopic(t)
		if err != nil {
			return err
		}
	}

	if s.cfg.ListenValidatorData {
		for t := range validatorMap {
			err := s.subscribeTopic(t)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *Service) subscribeTopic(name string) error {
	if _, exists := s.topics[name]; exists {
		return errors.Errorf("topic %s already exists", name)
	}

	if _, exists := s.subs[name]; exists {
		return errors.Errorf("subscription for topic %s already exists", name)
	}

	topic, err := s.pubsub.Join(name)
	if err != nil {
		return errors.Wrap(err, "can't join to topic")

	}

	sub, err := topic.Subscribe()
	if err != nil {
		return errors.Wrap(err, "can't subscribe to topic")
	}

	s.topics[name] = topic
	s.subs[name] = sub

	return nil
}

func (s *Service) Publish(topicName string, message []byte) error {
	s.topicLock.Lock()
	defer s.topicLock.Unlock()

	if len(s.peerStore.Connected()) == 0 {
		return nil
	}

	topic, exists := s.topics[topicName]
	if !exists {
		return errors.New("undefined topic")
	}

	failCounter := 0
	for {
		if len(topic.ListPeers()) > 0 {
			return topic.Publish(s.ctx, message)
		}

		// avoid blocking
		// todo fix events, create unblocking send
		if failCounter == failPublishLimit {
			return errors.New("no topic subscribers")
		}

		log.Debugf("Try public %s...", topicName)

		select {
		case <-s.ctx.Done():
			return errors.Wrap(s.ctx.Err(), "no peers found to publish message")
		default:
			time.Sleep(500 * time.Millisecond)
			failCounter++
		}
	}
}

func (s *Service) logID() {
	if len(s.host.Addrs()) == 0 {
		return
	}

	log.Infof("Start listen on %s/p2p/%s", s.host.Addrs()[0].String(), s.id.Pretty())
}

func (s *Service) receiveMessage(msg *pubsub.Message, isValidatorMessage bool) {
	n := Notty{
		Data:  msg.Data,
		Topic: *msg.Topic,
		From:  msg.ReceivedFrom.String(),
	}

	// send event
	if isValidatorMessage {
		s.validatorNotifier.Send(n)
	} else {
		s.notifier.Send(n)
	}
}

func (s *Service) Notifier() *events.Bus {
	return &s.notifier
}

func (s *Service) ValidatorNotifier() *events.Bus {
	return &s.validatorNotifier
}

func (s *Service) AddConnectionHandlers(connectHandler, disconnectHandler ConnectionHandler) {
	s.host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			remotePeer := conn.RemotePeer()

			// it should be non-blocking
			go func() {
				log.Debugf("Connected to the peer %s", remotePeer.String())
				s.peerStore.Connect(remotePeer)

				if err := connectHandler(s.ctx, remotePeer); err != nil {
					log.Errorf("Can't exec connect handler with %s: %s", remotePeer, err)
				}
			}()

		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			remotePeer := conn.RemotePeer()

			go func() {
				log.Debugf("Disconnected peer %s", remotePeer.String())
				s.peerStore.Disconnect(remotePeer)
				if err := disconnectHandler(s.ctx, remotePeer); err != nil {
					log.Errorf("Can't exec disconnect handler with %s: %s", remotePeer, err)
				}
			}()
		},
	})
}

func (s *Service) SetStreamHandler(topic string, handler network.StreamHandler) {
	s.host.SetStreamHandler(protocol.ID(topic), handler)
}

func (s *Service) DecodeStream(r io.Reader, to ssz.Unmarshaler) error {
	msgLen, err := readVarint(r)
	if err != nil {
		return err
	}
	if msgLen > MaxChunkSize {
		return fmt.Errorf(
			"remaining bytes %d goes over the provided max limit of %d",
			msgLen,
			MaxChunkSize,
		)
	}
	msgMax, err := maxLength(msgLen)
	if err != nil {
		return err
	}
	limitedRdr := io.LimitReader(r, int64(msgMax))
	r = newBufferedReader(limitedRdr)
	defer bufReaderPool.Put(r)

	buf := make([]byte, msgLen)
	// Returns an error if less than msgLen bytes
	// are read. This ensures we read exactly the
	// required amount.
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return err
	}
	return to.UnmarshalSSZ(buf)
}

func (s *Service) EncodeStream(w io.Writer, msg ssz.Marshaler) (int, error) {
	if msg == nil {
		return 0, nil
	}
	b, err := msg.MarshalSSZ()
	if err != nil {
		return 0, err
	}
	if uint64(len(b)) > MaxChunkSize {
		return 0, fmt.Errorf(
			"size of encoded message is %d which is larger than the provided max limit of %d",
			len(b),
			MaxChunkSize,
		)
	}
	// write varint first
	_, err = w.Write(writeVarint(len(b)))
	if err != nil {
		return 0, err
	}
	return writeSnappyBuffer(w, b)
}

func (s *Service) PeerStore() *PeerStore {
	return s.peerStore
}

func (s *Service) CreateStream(ctx context.Context, msg ssz.Marshaler, topic string, pid peer.ID) (network.Stream, error) {
	// Apply max dial timeout when opening a new stream.
	ctx, cancel := context.WithTimeout(ctx, maxDialTimeout)
	defer cancel()

	stream, err := s.host.NewStream(ctx, pid, protocol.ID(topic))
	if err != nil {
		return nil, err
	}

	if _, err := s.EncodeStream(stream, msg); err != nil {
		_err := stream.Close()
		_ = _err
		log.Warnf("Close stream for %s", string(stream.Protocol()))
		return nil, err
	}

	// Close stream for writing.
	if err := stream.CloseWrite(); err != nil {
		log.Warnf("Close stream for %s", string(stream.Protocol()))
		_err := stream.Close()
		_ = _err
		return nil, err
	}

	return stream, nil
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
			case state.ConnectionHandlersReady:
				s.initialized <- struct{}{}
			case state.Synced:
				close(s.initialized)
				return
			default:
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}

func (s *Service) EnableValidatorMode() {
	s.cfg.ListenValidatorData = true
}
