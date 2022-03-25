package p2p

import (
	"context"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/sirupsen/logrus"
	"sync"
)

// TODO create feed for packets messaging

const blockTopic = "rdo-block"
const txTopic = "rdo-tx"

var log = logrus.WithField("prefix", "p2p")

var emptyMsg = []byte("Empty p2p message")
var maxDialTimeout = params.RaidoConfig().ResponseTimeout

type Config struct {
	Port           int
	BootstrapNodes []string
	PeerLimit      int
	DataDir        string
}

func NewService(cfg *Config) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())

	nodePrivKey, err := getNodeKey(cfg.DataDir)
	if err != nil {
		return nil, err
	}

	ipAddr, err := getIPaddr()
	if err != nil {
		return nil, err
	}

	opts, err := optionsList(nodePrivKey, ipAddr, cfg)
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
		//pubsub.WithPeerScore(peerScoringParams()), // TODO add peer score ?
		//pubsub.WithPeerScoreInspect(s.peerInspector, time.Minute),
		pubsub.WithGossipSubParams(pubsubGossipParam()),
	}

	gs, err := pubsub.NewGossipSub(ctx, netHost, psOpts...)
	if err != nil {
		return nil, err
	}

	srv := &Service{
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
	host    host.Host
	id      peer.ID
	pubsub  *pubsub.PubSub
	topics map[string]*pubsub.Topic
	subs   map[string]*pubsub.Subscription

	ctx    context.Context
	cancel context.CancelFunc
	topicLock sync.Mutex

	cfg *Config

	startFail error
}

func (s *Service) Start() {
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
		s.startFail = err
		return
	}

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
			log.Error(errors.Wrap(err, "error listening topic " + topic))
			return
		}

		if msg.ReceivedFrom == id {
			continue
		}

		// TODO send message to the special feed

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
		blockTopic,
		txTopic,
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
		return errors.Wrap(err, "error publish message")
	}

	return nil
}