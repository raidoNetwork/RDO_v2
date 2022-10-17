package sync

import (
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	"github.com/raidoNetwork/RDO_v2/validator/types"
)

func (s *Service) listenValidatorTopics() {
	validatorNotification := make(chan p2p.Notty, notificationsGossipCount)
	subs := s.cfg.P2P.ValidatorNotifier().Subscribe(validatorNotification)
	defer subs.Unsubscribe()

	for{
		select{
		case <-s.ctx.Done():
			return
		case notty := <-validatorNotification:
			switch notty.Topic {
			case p2p.ProposalTopic:
				block, err := serialize.UnmarshalBlock(notty.Data)
				if err != nil {
					log.Errorf("Error unmarshaling block: %s", err)
					break
				}

				s.cfg.Validator.ProposeFeed.Send(block)
				receivedMessages.WithLabelValues(p2p.ProposalTopic).Inc()
			case p2p.AttestationTopic:
				att, err := serialize.UnmarshalAttestation(notty.Data)
				if err != nil {
					log.Errorf("Error unmarshaling attestation: %s", err)
					break
				}

				s.cfg.Validator.AttestationFeed.Send(att)
				receivedMessages.WithLabelValues(p2p.AttestationTopic).Inc()
			default:
				log.Warnf("Unsupported notification %s", notty.Topic)
			}
		}
	}
}

func (s *Service) gossipValidatorMessages() {
	proposeEvent := make(chan *prototype.Block, 1)
	attEvent := make(chan *types.Attestation, 5)

	proposeSub := s.cfg.Validator.ProposeFeed.Subscribe(proposeEvent)
	attSub := s.cfg.Validator.AttestationFeed.Subscribe(attEvent)

	defer func() {
		proposeSub.Unsubscribe()
		attSub.Unsubscribe()
	} ()

	for{
		select{
		case block := <-proposeEvent:
			raw, err := serialize.MarshalBlock(block)
			if err != nil {
				log.Errorf("Error marshaling block: %s", err)
				continue
			}

			err = s.cfg.P2P.Publish(p2p.ProposalTopic, raw)
			if err != nil {
				log.Errorf("Error sending proposed block: %s", err)
			}
		case att := <-attEvent:
			raw, err := serialize.MarshalAttestation(att)
			if err != nil {
				log.Errorf("Error marshaling attestation: %s", err)
				continue
			}

			err = s.cfg.P2P.Publish(p2p.AttestationTopic, raw)
			if err != nil {
				log.Errorf("Error sending attestation: %s", err)
			}

			log.Debugf("Publish attestation for block #%d %s", att.Block.Num, common.Encode(att.Block.Hash))
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Service) EnableValidatorMode() {
	s.cfg.Validator.Enabled = true
}
