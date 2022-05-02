package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

const (
	discoveryTag = "rdo-node-net"
	discoveryProtocol = "rdo-world"
)

func (s *Service) setupDiscovery() error {
	if err := s.dht.Bootstrap(s.ctx); err != nil {
		return err
	}

	// annonce node in the network
	routing := discovery.NewRoutingDiscovery(s.dht)
	discovery.Advertise(s.ctx, routing, discoveryTag)

	// start peer search loop
	go s.findPeers(routing)

	return nil
}

func (s *Service) findPeers(routing *discovery.RoutingDiscovery){
	peers, err := routing.FindPeers(s.ctx, discoveryTag)
	if err != nil {
		log.WithError(err).Error("Error peer search.")
		return
	}

	for p := range peers {
		s.HandlePeerFound(p)
	}

	// tests
	log.Error("Peer search channel closed")
}

func (s *Service) HandlePeerFound(p peer.AddrInfo){
	if p.ID == s.host.ID() {
		return
	}

	log.Infof("Found new node %s/p2p/%s", p.Addrs[0].String(), p.ID.Pretty())

	err := s.connectPeer(p)
	if err != nil {
		log.Errorf("Error connecting to new peer: %s", err)
	}
}

func (s *Service) dhtOpts() []dht.Option {
	dopts := []dht.Option{
		dht.ProtocolPrefix(discoveryProtocol),
		dht.BootstrapPeers(s.getPeerInfo(s.cfg.BootstrapNodes)...),
		dht.Mode(dht.ModeAutoServer),
	}

	return dopts
}