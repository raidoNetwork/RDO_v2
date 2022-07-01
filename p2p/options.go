package p2p

import (
	"fmt"
	"github.com/libp2p/go-libp2p"
	phost "github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/libp2p/go-libp2p/p2p/transport/tcp"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/version"
	"time"
)

func (s *Service) optionsList(nodePrivKey *nodeKey, ip string, cfg *Config) ([]libp2p.Option, error) {
	maddr := fmt.Sprintf("/ip4/%s/tcp/%d", ip, cfg.Port)

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(maddr),
		libp2p.Identity(nodePrivKey.p2pKey),
		libp2p.UserAgent(version.Version()),
		libp2p.Transport(tcp.NewTCPTransport),
		libp2p.Security(noise.ID, noise.New),
		libp2p.DisableRelay(),
		libp2p.Routing(func(h phost.Host) (routing.PeerRouting, error) {
			var err error
			s.dht, err = dht.New(s.ctx, h, s.dhtOpts()...)
			return s.dht, err
		}),
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
	gParams.Dlo = 6
	gParams.D = 8
	gParams.HeartbeatInterval = 700 * time.Millisecond
	gParams.HistoryLength = 6
	gParams.HistoryGossip = 3

	return gParams
}

func peerScoringParams() (*pubsub.PeerScoreParams, *pubsub.PeerScoreThresholds) {
	thresholds := &pubsub.PeerScoreThresholds{
		GossipThreshold:             -4000,
		PublishThreshold:            -8000,
		GraylistThreshold:           -16000,
		AcceptPXThreshold:           100,
		OpportunisticGraftThreshold: 5,
	}
	scoreParams := &pubsub.PeerScoreParams{
		Topics:        make(map[string]*pubsub.TopicScoreParams),
		TopicScoreCap: 32.72,
		AppSpecificScore: func(p peer.ID) float64 {
			return 0
		},
		AppSpecificWeight:           1,
		IPColocationFactorWeight:    -35.11,
		IPColocationFactorThreshold: 10,
		IPColocationFactorWhitelist: nil,
		BehaviourPenaltyWeight:      -15.92,
		BehaviourPenaltyThreshold:   6,
		BehaviourPenaltyDecay:       0.5,
		DecayInterval:               slotDuration(),
		DecayToZero:                 0.01,
		//RetainScore:                 oneHundredEpochs,
	}
	return scoreParams, thresholds
}

func slotDuration() time.Duration {
	return time.Duration(params.RaidoConfig().SlotTime) * time.Second
}