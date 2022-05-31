package attestation

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/attestation"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/staking"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/rdochain"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"time"
)

type Config struct {
	TxFeed events.Feed
	StateFeed     events.Feed
	EnableMetrics bool
	Blockchain    *rdochain.Service
}

func NewService(ctx context.Context, cfg *Config) (*Service, error) {
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

	// new tx pool
	txPool := NewTxPool(ctx, validator, &PoolConfig{
		MinimalFee: chainConfig.MinimalFee,
		BlockSize:  chainConfig.BlockSize,
		TxFeed:     cfg.TxFeed,
	})

	stateEvent := make(chan state.State, 1)
	cfg.StateFeed.Subscribe(stateEvent)

	srv := &Service{
		txPool: txPool,
		stakePool: stakePool,
		validator: validator,
		stateEvent: stateEvent,
	}

	return srv, nil
}

type Service struct{
	txPool *TxPool
	stakePool consensus.StakePool
	validator consensus.Validator
	stateEvent chan state.State
}

func (s *Service) Start(){
	// lock for sync
	err := s.waitSyncing()
	if err != nil {
		log.WithError(err).Error("Can't start attestation service")
		return
	}

	// listen events for forge error
	go s.lookForgeError()

	// start tx pool work
	go s.txPool.ReadingLoop()
}

func (s *Service) Status() error {
	return nil
}

func (s *Service) Stop() error {
	s.txPool.StopWriting()
	log.Info("Stop tx pool")

	return nil
}


// SendRawTx implements PoolAPI for gRPC gateway
func (s *Service) SendRawTx(tx *prototype.Transaction) error {
	return s.txPool.SendRawTx(tx)
}

// GetFee returns minimal fee needed to insert transaction in the block.
func (s *Service) GetFee() uint64 {
	return s.txPool.GetFee()
}

// GetPendingTransactions returns list of pending transactions.
func (s *Service) GetPendingTransactions() ([]*prototype.Transaction, error) {
	return s.txPool.GetPendingTransactions()
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

func (s *Service) lookForgeError() {
	for st := range s.stateEvent {
		if st == state.ForgeFailed {
			s.txPool.catchForgeError()
			return
		}
	}
}

func (s *Service) waitSyncing() error {
	for st := range s.stateEvent {
		if st == state.Synced {
			return s.stakePool.LoadData()
		}

		time.Sleep(500 * time.Millisecond)
	}

	return nil
}