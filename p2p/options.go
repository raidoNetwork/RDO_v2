package p2p

import (
	"fmt"
	"github.com/libp2p/go-libp2p"
	noise "github.com/libp2p/go-libp2p-noise"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-tcp-transport"
	"github.com/pkg/errors"
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
	}

	return opts, nil
}

func msgId(pmsg *pubsubpb.Message) string {
	size := pmsg.Size()
	data := make([]byte, 0, size)
	_, err := pmsg.MarshalTo(data)
	if err != nil {
		log.Error(errors.Wrap(err, "Message marshal error"))
		data = emptyMsg
	}

	return string(crypto.Keccak256(data))
}

func pubsubGossipParam() pubsub.GossipSubParams {
	gParams := pubsub.DefaultGossipSubParams()
	gParams.Dlo = 6
	gParams.D = 8
	gParams.HeartbeatInterval = 700 * time.Millisecond
	gParams.HistoryLength = 6
	gParams.HistoryGossip = 3

	return gParams
}