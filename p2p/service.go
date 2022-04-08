package p2p

import (
	"context"
	"github.com/libp2p/go-libp2p"
	phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
)

const BlockTopic = "rdo-block"
const TxTopic = "rdo-transactions"

var log = logrus.WithField("prefix", "p2p")
var maxDialTimeout = params.RaidoConfig().ResponseTimeout

type Config struct {
	Host           string
	Port           int
	BootstrapNodes []string
	PeerLimit      int
	DataDir        string
}

func NewService(ctx context.Context, cfg *Config) (srv *Service, err error) {
	ctx, cancel := context.WithCancel(ctx)

	nodePrivKey, err := getNodeKey(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	host := cfg.Host
	if host == "" {
		host, err = getIPaddr()
		if err != nil {
			return nil, err
		}
	} else{
		hostIp := net.ParseIP(host)
		if hostIp.To4() == nil {
			return nil, errors.Errorf("Invalid host IP address given %s", host)
		}
	}

	opts, err := optionsList(nodePrivKey, host, cfg)
	if err != nil {
		return nil, err
	}

	netHost, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}

	psOpts := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictNoSign),
		pubsub.WithNoAuthor(),
		pubsub.WithMessageIdFn(msgId),
		pubsub.WithPeerOutboundQueueSize(256),
		pubsub.WithValidateQueueSize(256),
		pubsub.WithGossipSubParams(pubsubGossipParam()),
	}

	gs, err := pubsub.NewGossipSub(ctx, netHost, psOpts...)
	if err != nil {
		return nil, err
	}

	srv = &Service{
		nodeKey: nodePrivKey,
		host:    netHost,
		pubsub:  gs,
		ctx:     ctx,
		cancel:  cancel,
		cfg:     cfg,
		id:      netHost.ID(),
		topics:  map[string]*pubsub.Topic{},
		subs:	 map[string]*pubsub.Subscription{},
	}

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
}

func (s *Service) Start() {
	// print host p2p address
	s.logID()

	// connect to bootstrap nodes
	s.connectPeers()

	// start new peer search
	err := s.setupDiscovery()
	if err != nil {
		s.startFail = err
		return
	}

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
	for _, addr := range s.cfg.BootstrapNodes {
		if addr == "" {
			continue
		}

		go s.connectPeerWithAddr(addr)
	}
}

func (s *Service) connectPeerWithAddr(addr string) error {
	peerInfo, err := peer.AddrInfoFromString(addr)
	if err != nil {
		log.Errorf("Error parsing peer info: %s", err)
		return err
	}

	return s.connectPeer(*peerInfo)
}

func (s *Service) connectPeer(info peer.AddrInfo) error {
	ctx, cancel := context.WithTimeout(s.ctx, maxDialTimeout)
	defer cancel()

	if err := s.host.Connect(ctx, info); err != nil {
		log.Errorf("Error connection to the peer %v: %s", info.Addrs, err)
		return err
	}

	return nil
}

func (s *Service) readMessages(){
	log.Info("Start listening messages")

	topics := s.activeTopics()
	for _, t := range topics {
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
			log.WithError(err).Error("error listening topic " + topic)
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
	topics := s.activeTopics()

	for _, t := range topics {
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

func (s *Service) activeTopics() []string {
	topics := []string{
		BlockTopic,
		TxTopic,
	}

	return topics
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

	log.Infof("Publish message to %s", topicName)

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
	}

	// send event
	s.notifier.Send(n)
}

func (s *Service) Notifier() *events.Bus {
	return &s.notifier
}