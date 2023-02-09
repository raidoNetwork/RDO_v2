package sync

import (
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	"github.com/raidoNetwork/RDO_v2/validator/types"
)

const (
	seedGossipCount        = 50
	proposalGossipCount    = 5
	attestationGossipCount = 50
)

func (s *Service) listenValidatorTopics() {
	seedNotification := make(chan p2p.Notty, seedGossipCount)
	proposalNotification := make(chan p2p.Notty, proposalGossipCount)
	attestationNotification := make(chan p2p.Notty, attestationGossipCount)
	subSeed := s.cfg.P2P.ValidatorSeedNotifier().Subscribe(seedNotification)
	subAtt := s.cfg.P2P.ValidatorAttNotifier().Subscribe(attestationNotification)
	subProposal := s.cfg.P2P.ValidatorProposalNotifier().Subscribe(proposalNotification)
	defer func() {
		subSeed.Unsubscribe()
		subAtt.Unsubscribe()
		subProposal.Unsubscribe()
	}()

	for {
		select {
		case <-s.ctx.Done():
			return
		case notty := <-proposalNotification:
			block, err := serialize.UnmarshalBlock(notty.Data)
			if err != nil {
				log.Errorf("Error unmarshaling block: %s", err)
				break
			}

			s.cfg.Validator.ProposeFeed.Send(block)
			receivedMessages.WithLabelValues(p2p.ProposalTopic).Inc()
		case notty := <-attestationNotification:
			att, err := serialize.UnmarshalAttestation(notty.Data)
			if err != nil {
				log.Errorf("Error unmarshaling attestation: %s", err)
				break
			}

			s.cfg.Validator.AttestationFeed.Send(att)
			receivedMessages.WithLabelValues(p2p.AttestationTopic).Inc()

		case notty := <-seedNotification:
			seed, err := serialize.UnmarshalSeed(notty.Data)
			if err != nil {
				log.Errorf("Error unmarshaling seed: %s", err)
				break
			}
			s.cfg.Validator.SeedFeed.Send(seed)
			receivedMessages.WithLabelValues(p2p.SeedTopic).Inc()
		}
	}
}

func (s *Service) gossipValidatorMessages() {
	proposeEvent := make(chan *prototype.Block, 5)
	attEvent := make(chan *types.Attestation, 100)
	seedEvent := make(chan *prototype.Seed, 100)

	proposeSub := s.cfg.Validator.ProposeFeed.Subscribe(proposeEvent)
	attSub := s.cfg.Validator.AttestationFeed.Subscribe(attEvent)
	seedSub := s.cfg.Validator.SeedFeed.Subscribe(seedEvent)

	defer func() {
		proposeSub.Unsubscribe()
		attSub.Unsubscribe()
		seedSub.Unsubscribe()
	}()

	for {
		select {
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
		case seed := <-seedEvent:
			raw, err := serialize.MarshalSeed(seed)
			if err != nil {
				log.Errorf("Error marshaling seed: %s", err)
				continue
			}
			err = s.cfg.P2P.Publish(p2p.SeedTopic, raw)
			if err != nil {
				log.Errorf("Error sending seed: %s", err)
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
