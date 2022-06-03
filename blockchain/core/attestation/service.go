package attestation

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/attestation"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/staking"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/rdochain"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
	"time"
)

type Config struct {
	TxFeed events.Feed
	StateFeed     events.Feed
	EnableMetrics bool
	Blockchain    *rdochain.Service
}

func NewService(parentCtx context.Context, cfg *Config) (*Service, error) {
	chainConfig := params.RaidoConfig()

	stakeAmount := chainConfig.StakeSlotUnit * chainConfig.RoiPerRdo
	slotTime := time.Duration(chainConfig.SlotTime) * time.Second

	// create new staking pool
	stakePool := staking.NewPool(cfg.Blockchain, chainConfig.ValidatorRegistryLimit, chainConfig.RewardBase, stakeAmount)

	validatorCfg := attestation.CryspValidatorConfig{
		SlotTime:               slotTime,
		MinFee:                 chainConfig.MinimalFee,
		StakeUnit:              stakeAmount,
		ValidatorRegistryLimit: chainConfig.ValidatorRegistryLimit,
		EnableMetrics:          cfg.EnableMetrics,
	}

	// new block and tx validator
	validator := attestation.NewCryspValidator(cfg.Blockchain, stakePool, &validatorCfg)

	txPool := NewPool(&PoolSettings{
		Validator: validator,
		MinimalFee: chainConfig.MinimalFee,
	})

	txEvent := make(chan *types.Transaction, maxTxCount)
	stateEvent := make(chan state.State, 1)
	ctx, cancel := context.WithCancel(parentCtx)

	srv := &Service{
		txPool: txPool,
		stakePool: stakePool,
		validator: validator,
		stateEvent: stateEvent,
		txEvent: txEvent,
		cfg: cfg,
		ctx: ctx,
		cancel: cancel,
	}

	return srv, nil
}

type Service struct{
	txPool *Pool
	stakePool consensus.StakePool
	validator consensus.Validator

	stateEvent chan state.State
	txEvent chan *types.Transaction

	cfg *Config

	ctx context.Context
	cancel context.CancelFunc
}

func (s *Service) Start(){
	// lock for sync
	err := s.waitSyncing()
	if err != nil {
		log.WithError(err).Error("Stake pool error")
		return
	}

	// start tx pool work
	go s.txListener()
}

func (s *Service) txListener() {
	sub := s.cfg.TxFeed.Subscribe(s.txEvent)
	defer sub.Unsubscribe()

	for {
		select {
		case tx := <-s.txEvent:
			err := s.txPool.Insert(tx)
			if err != nil {
				log.Error(errors.Wrap(err , "Transaction listener error"))
			}
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *Service) Status() error {
	return nil
}

func (s *Service) Stop() error {
	s.cancel()

	log.Info("Stop Attestation service")
	return nil
}


// SendRawTx implements PoolAPI for gRPC gateway
func (s *Service) SendRawTx(tx *prototype.Transaction) error {
	_, err := tx.MarshalSSZ()
	if err != nil {
		return status.Error(17, "Transaction has bad format")
	}

	s.cfg.TxFeed.Send(types.NewTransaction(tx))

	return nil
}

// GetFee returns minimal fee price needed to insert transaction in the block.
func (s *Service) GetFee() uint64 {
	return s.txPool.GetFeePrice()
}

// GetPendingTransactions returns list of pending transactions.
func (s *Service) GetPendingTransactions() ([]*prototype.Transaction, error) {
	queue := s.txPool.GetQueue()
	res := make([]*prototype.Transaction, 0, len(queue))

	for _, tx := range queue {
		res = append(res, tx.GetTx())
	}

	return res, nil
}

func (s *Service) GetServiceStatus() (string, error) {
	return "Unimplemented", nil
}

func (s *Service) StakePool() consensus.StakePool {
	return s.stakePool
}

func (s *Service) TxPool() consensus.TxPool {
	return s.txPool
}

func (s *Service) Validator() consensus.Validator {
	return s.validator
}

func (s *Service) waitSyncing() error {
	sub := s.cfg.StateFeed.Subscribe(s.stateEvent)
	defer sub.Unsubscribe()

	for st := range s.stateEvent {
		if st == state.Synced {
			return s.stakePool.LoadData()
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}