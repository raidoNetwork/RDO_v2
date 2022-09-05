package sync

import (
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	"github.com/raidoNetwork/RDO_v2/validator/types"
)

func (s *Service) listenValidatorTopics() {
	validatorNotification := make(chan p2p.Notty, notificationsGossipCount)
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

				log.Debugf("Receive block #%d", block.Num)

				s.cfg.ValidatorService.ProposeFeed().Send(block)
				receivedMessages.WithLabelValues(p2p.ProposalTopic).Inc()
			case p2p.AttestationTopic:
				att := &types.Attestation{}
				err := att.UnmarshalSSZ(notty.Data)
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
