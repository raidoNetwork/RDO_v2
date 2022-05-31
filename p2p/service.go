package p2p

import (
	"context"
	"github.com/libp2p/go-libp2p"
	phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/utils/async"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "p2p")
var maxDialTimeout = params.RaidoConfig().ResponseTimeout

type Config struct {
	Host           string
	Port           int
	BootstrapNodes []string
	DataDir        string
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
		nodeKey: nodePrivKey,
		ctx:     ctx,
		cancel:  cancel,
		cfg:     cfg,
		topics:  map[string]*pubsub.Topic{},
		subs:	 map[string]*pubsub.Subscription{},
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
	srv.id = netHost.ID()

	psOpts := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign),
		pubsub.WithNoAuthor(),
		pubsub.WithMessageIdFn(msgId),
		pubsub.WithPeerOutboundQueueSize(256),
		pubsub.WithValidateQueueSize(256),
		pubsub.WithGossipSubParams(pubsubGossipParam()),
		pubsub.WithPeerScore(peerScoringParams()),
		pubsub.WithSubscriptionFilter(srv),
	}

	gs, err := pubsub.NewGossipSub(ctx, netHost, psOpts...)
	if err != nil {
		return nil, err
	}

	srv.pubsub = gs

	async.WithInterval(srv.ctx, 10 * time.Second, func() {
		srv.updateMetrics()
	})

	return srv, nil
}

type Service struct {
	nodeKey *nodeKey
	host    phost.Host
	id      peer.ID
	pubsub  *pubsub.PubSub
	topics map[string]*pubsub.Topic
	subs   map[string]*pubsub.Subscription

	ctx    context.Context
	cancel context.CancelFunc
	topicLock sync.Mutex

	cfg *Config

	startFail error

	notifier events.Bus

	// discovery
	dht *dht.IpfsDHT
}

func (s *Service) Start() {
	// print host p2p address
	s.logID()

	s.addConnectionHandlers()

	// start new peer search
	err := s.setupDiscovery()
	if err != nil {
		s.startFail = err
		return
	}

	// connect to bootstrap nodes
	s.connectPeers()

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
	if err := s.host.Close(); err != nil {
		return err
	}

	s.cancel()

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
		go s.connectPeer(info)
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

	if err := s.host.Connect(ctx, info); err != nil {
		log.Errorf("Error connection to the peer %v: %s", info.Addrs, err)
		return err
	}

	log.Infof("Connect to the %s", info.String())

	return nil
}

func (s *Service) readMessages(){
	log.Info("Start listening messages")

	for t := range topicMap {
		go s.listenTopic(t)
	}
}

func (s *Service) listenTopic(topic string){
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

		go s.receiveMessage(msg)
	}
}

func (s *Service) SubscribeAll() error {
	for t := range topicMap {
		err := s.subscribeTopic(t)
		if err != nil {
			return err
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

	topic, exists := s.topics[topicName]
	if !exists {
		return errors.New("undefined topic")
	}

	err := topic.Publish(s.ctx, message)
	if err != nil {
		return errors.Wrap(err, "error Publish message")
	}

	return nil
}

func (s *Service) logID(){
	if len(s.host.Addrs()) == 0 {
		return
	}

	log.Infof("Start listen on %s/p2p/%s", s.host.Addrs()[0].String(), s.id.Pretty())
}

func (s *Service) receiveMessage(msg *pubsub.Message){
	n := Notty{
		Data: msg.Data,
		Topic: *msg.Topic,
		From: msg.ReceivedFrom.String(),
	}

	// send event
	s.notifier.Send(n)
}

func (s *Service) Notifier() *events.Bus {
	return &s.notifier
}

func (s *Service) addConnectionHandlers() {
	s.host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			remotePeer := conn.RemotePeer()
			peerData := s.host.Peerstore().PeerInfo(remotePeer)
			if len(peerData.Addrs) == 0 {
				log.Debugf("Connected to the peer %s", remotePeer.String())
				p2pPeerMap.WithLabelValues("Connected").Inc()
			} else {
				log.Debugf("Reconnected to the peer %s", remotePeer.String())
				p2pPeerMap.WithLabelValues("Reconnected").Inc()
			}
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			remotePeer := conn.RemotePeer()
			log.Debugf("Disconnected peer %s", remotePeer.String())
			p2pPeerMap.WithLabelValues("Connected").Dec()
			p2pPeerMap.WithLabelValues("Disconnected").Inc()
		},
	})
}