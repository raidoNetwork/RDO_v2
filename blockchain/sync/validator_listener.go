package sync

import (
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	"github.com/raidoNetwork/RDO_v2/validator"
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

				log.Debugf("Receive proposed block #%d", block.Num)

				s.cfg.ValidatorService.ProposeFeed().Send(block)
				receivedMessages.WithLabelValues(p2p.ProposalTopic).Inc()
			case p2p.AttestationTopic:
				att, err := serialize.UnmarshalAttestation(notty.Data)
				if err != nil {
					log.Errorf("Error unmarshaling attestation: %s", err)
					break
				}

				log.Debugf("Receive attestation fro block #%d", att.Block.Num)

				s.cfg.ValidatorService.AttestationFeed().Send(att)
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

	proposeSub := s.cfg.ValidatorService.ProposeFeed().Subscribe(proposeEvent)
	attSub := s.cfg.ValidatorService.AttestationFeed().Subscribe(attEvent)

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
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Service) EnableValidatorMode(validator *validator.Service) {
	s.cfg.ValidatorService = validator
}
