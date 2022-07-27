package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	discovery "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	discoveryUtil "github.com/libp2p/go-libp2p/p2p/discovery/util"
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
	discoveryUtil.Advertise(s.ctx, routing, discoveryTag)

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
}

func (s *Service) HandlePeerFound(p peer.AddrInfo){
	if p.ID == s.host.ID() {
		return
	}

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