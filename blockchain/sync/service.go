package sync

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "syncBlocks")

const (
	txGossipCount = 200
	blockGossipCount = 100
	stateCount = 1
	notificationsGossipCount = txGossipCount + blockGossipCount
)

type GossipPublisher interface{
	Publish(string, []byte) error

	Notifier() *events.Bus
}

type BlockchainInfo interface{
	GetBlockCount() uint64
}

type Config struct{
	BlockFeed *events.Bus
	TxFeed	  *events.Bus
	StateFeed *events.Bus
	Publisher  GossipPublisher
	Blockchain BlockchainInfo
}

func NewService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	srv := &Service{
		cfg: cfg,
		txEvent: make(chan *types.Transaction, txGossipCount),
		blockEvent: make(chan *prototype.Block, blockGossipCount),
		stateEvent: make(chan state.State, stateCount),
		notification: make(chan p2p.Notty, notificationsGossipCount),
		ctx: ctx,
		cancel: cancel,
	}

	// subscribe on new events
	srv.subscribeEvents()

	return srv
}

type Service struct{
	cfg *Config
	bc BlockchainInfo

	txEvent chan *types.Transaction
	blockEvent chan *prototype.Block
	stateEvent chan state.State
	notification chan p2p.Notty

	ctx 	context.Context
	cancel  context.CancelFunc
}

func (s *Service) Start(){
	// syncBlocks with network
	s.syncData()

 	// gossip new blocks and transactions
	go s.gossipEvents()

	// listen incoming data
	go s.listenIncoming()
}

func (s *Service) gossipEvents(){
	for{
		select{
			case block := <-s.blockEvent:
				raw, err := serialize.MarshalBlock(block)
				if err != nil {
					log.Errorf("Error marshaling block: %s", err)
					continue
				}

				err = s.cfg.Publisher.Publish(p2p.BlockTopic, raw)
				if err != nil {
					log.Errorf("Error sending block: %s", err)
				}
			case td := <-s.txEvent:
				raw, err := serialize.MarshalTx(td.GetTx())
				if err != nil {
					log.Errorf("Error marshaling transaction: %s", err)
					continue
				}

				err = s.cfg.Publisher.Publish(p2p.TxTopic, raw)
				if err != nil {
					log.Errorf("Error sending transaction: %s", err)
				}
			case <-s.ctx.Done():
				return
		}
	}
}

func (s *Service) Stop() error {
	// cancel context
	s.cancel()

	return nil
}

func (s *Service) Status() error {
	return nil
}


func (s *Service) syncData(){
	sub := s.cfg.StateFeed.Subscribe(s.stateEvent)
	defer sub.Unsubscribe()

	for {
		select{
		case <-s.ctx.Done():
			return
		case st := <-s.stateEvent:
			switch st {
			case state.LocalSynced:
				s.syncBlocks() // syncBlocks with network
			case state.Synced:
				return
			default:
				log.Infof("Unknown state event %d", st)
				return
			}
		}
	}
}

func (s *Service) listenIncoming() {
	for{
		select{
			case <-s.ctx.Done():
				return
			case notty := <-s.notification:
				switch notty.Topic {
				case p2p.BlockTopic:
					block, err := serialize.UnmarshalBlock(notty.Data)
					if err != nil {
						log.Errorf("Error unmarshaling block: %s", err)
						break
					}

					log.Infof("Block received %d", block.Num)

					//s.cfg.BlockFeed.Send(block)
				case p2p.TxTopic:
					tx, err := serialize.UnmarshalTx(notty.Data)
					if err != nil {
						log.Errorf("Error unmarshaling transaction: %s", err)
						break
					}

					s.cfg.TxFeed.Send(types.NewTxData(tx))
				default:
					log.Warnf("Unsupported notification %s", notty.Topic)
				}
		}
	}
}

func (s *Service) syncBlocks(){
	log.Println("Data synced")

	// todo implement sync logic

    // send ready event
	s.cfg.StateFeed.Send(state.Synced)
}

// subscribeEvents on updates
func (s *Service) subscribeEvents(){
	s.cfg.Publisher.Notifier().Subscribe(s.notification)
	s.cfg.TxFeed.Subscribe(s.txEvent)
	s.cfg.BlockFeed.Subscribe(s.blockEvent)
}