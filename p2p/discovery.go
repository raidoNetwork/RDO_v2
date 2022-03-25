package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

const discoveryTag = "rdo-node-net"

func (s *Service) setupDiscovery() error {
	disc := mdns.NewMdnsService(s.host, discoveryTag, s)
	return disc.Start()
}

func (s *Service) HandlePeerFound(p peer.AddrInfo){
	log.Infof("Found new node %s", p.ID.Pretty())

	err := s.connectPeer(p)
	if err != nil {
		log.Errorf("Error connecting to new peer: %s", err)
	}
}
