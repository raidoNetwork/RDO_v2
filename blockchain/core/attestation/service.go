package attestation

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/attestation"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/staking"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/rdochain"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"google.golang.org/grpc/status"
)

var _ shared.Service = (*Service)(nil)

type Config struct {
	TxFeed        *events.Feed
	StateFeed     *events.Feed
	EnableMetrics bool
	Blockchain    *rdochain.Service
}

func NewService(parentCtx context.Context, cfg *Config) (*Service, error) {
	chainConfig := params.RaidoConfig()

	stakeAmount := chainConfig.StakeSlotUnit * chainConfig.RoiPerRdo
	slotTime := time.Duration(chainConfig.SlotTime) * time.Second

	// create new staking pool
	stakePool := staking.NewPool(cfg.Blockchain, chainConfig.ValidatorRegistryLimit, stakeAmount)

	validatorCfg := attestation.CryspValidatorConfig{
		SlotTime:               slotTime,
		MinFee:                 chainConfig.MinimalFee,
		StakeUnit:              stakeAmount,
		ValidatorRegistryLimit: chainConfig.ValidatorRegistryLimit,
		EnableMetrics:          cfg.EnableMetrics,
		BlockSize:              chainConfig.BlockSize,
	}

	// new block and tx validator
	validator := attestation.NewCryspValidator(cfg.Blockchain, stakePool, &validatorCfg)

	// new tx pool
	txPool := NewPool(&PoolSettings{
		Validator:  validator,
		MinimalFee: chainConfig.MinimalFee,
	})

	txEvent := make(chan *types.Transaction, maxTxCount)
	stateEvent := make(chan state.State, 1)
	ctx, cancel := context.WithCancel(parentCtx)

	srv := &Service{
		txPool:     txPool,
		stakePool:  stakePool,
		validator:  validator,
		stateEvent: stateEvent,
		txEvent:    txEvent,
		cfg:        cfg,
		ctx:        ctx,
		cancel:     cancel,
	}

	return srv, nil
}

type Service struct {
	txPool    *Pool
	stakePool consensus.StakePool
	validator consensus.Validator

	stateEvent chan state.State
	txEvent    chan *types.Transaction

	cfg *Config

	ctx    context.Context
	cancel context.CancelFunc
}

func (s *Service) Start() {
	// wait for sync
	err := s.waitSyncing()
	if err != nil {
		panic(errors.Wrap(err, "Stake pool error"))
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
				log.Debug(errors.Wrap(err, "Transaction listener error"))
			}
		case <-s.ctx.Done():
			log.Debugf("Stop Transaction listener loop")
			return
		}
	}
}

func (s *Service) Status() error {
	return nil
}

func (s *Service) Stop() error {
	log.Info("Stop Attestation service")

	s.cancel()

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
	stateFeed := s.cfg.StateFeed

	err := s.stakePool.Init()
	if err != nil {
		return err
	}

	stateFeed.Send(state.StakePoolInitialized)

	return nil
}

func (s *Service) StakersLimitReached(tx *types.Transaction) error {
	return s.txPool.CheckMaxStakers(tx)
}

func (s *Service) IsNodeValidator(tx *types.Transaction) error {
	return s.stakePool.IsNodeValidator(tx)
}

func (s *Service) ListValidators() []string {
	return s.stakePool.ListValidators()
}
