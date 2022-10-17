package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"strings"
)

const subscriptionsLimit = 10

func (s *Service) CanSubscribe(topic string) bool {
	words := strings.Split(topic, "/")
	if len(words) != 3 {
		return false
	}

	if words[0] != "" || words[1] != "raido" {
		return false
	}

	_, exists := topicMap[topic]

	if s.cfg.ListenValidatorData && !exists {
		_, exists = validatorMap[topic]
	}

	return exists
}

func(s *Service) FilterIncomingSubscriptions(id peer.ID, subs []*pb.RPC_SubOpts) ([]*pb.RPC_SubOpts, error){
	if len(subs) > subscriptionsLimit {
		return nil, pubsub.ErrTooManySubscriptions
	}

	return pubsub.FilterSubscriptions(subs, s.CanSubscribe), nil
}


