package p2p

import (
	"fmt"
	"github.com/libp2p/go-libp2p"
	noise "github.com/libp2p/go-libp2p-noise"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-tcp-transport"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/version"
	"time"
)

func optionsList(nodePrivKey *nodeKey, ip string, cfg *Config) ([]libp2p.Option, error) {
	maddr := fmt.Sprintf("/ip4/%s/tcp/%d", ip, cfg.Port)

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(maddr),
		libp2p.Identity(nodePrivKey.p2pKey),
		libp2p.UserAgent(version.Version()),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Security(noise.ID, noise.New),
		libp2p.DisableRelay(),
	}

	return opts, nil
}

func msgId(pmsg *pubsubpb.Message) string {
	topic := *pmsg.Topic
	size := len(pmsg.Data) + len(topic)

	data := make([]byte, 0, size)
	data = append(data, topic...)
	data = append(data, pmsg.Data...)

	id := string(crypto.Keccak256(data)[:20])
	return id
}

func pubsubGossipParam() pubsub.GossipSubParams {
	gParams := pubsub.DefaultGossipSubParams()
	gParams.Dlo = 1 //6 test value
	gParams.D = 1 //8 test value
	gParams.HeartbeatInterval = 700 * time.Millisecond
	gParams.HistoryLength = 6
	gParams.HistoryGossip = 3

	return gParams
}