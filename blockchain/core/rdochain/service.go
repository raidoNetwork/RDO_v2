package rdochain

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/attestation"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/miner"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/txpool"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc/status"
	"sync"
	"time"
)

const generatorInterval = 500 * time.Millisecond

// NewService creates new ChainService
func NewService(cliCtx *cli.Context, kv db.BlockStorage, sql db.OutputStorage) (*Service, error) {
	statFlag := cliCtx.Bool(flags.SrvStat.Name)
	expStatFlag := cliCtx.Bool(flags.SrvExpStat.Name)

	cfg := params.RaidoConfig()
	slotTime := time.Duration(cfg.SlotTime) * time.Second

	// create blockchain instance
	bc, err := NewBlockChain(kv, cliCtx)
	if err != nil {
		log.Errorf("Error creating blockchain: %s", err)
		return nil, err
	}

	// output manager
	outm := NewOutputManager(bc, sql, &OutputManagerConfig{
		ShowStat: statFlag,
	})

	// new block and tx validator
	validator := consensus.NewCryspValidator(bc, outm, statFlag, expStatFlag) // TODO remove it to another service

	// new tx pool
	txPool := txpool.NewTxPool(validator) // TODO remove it to another service

	// create new attestation validator
	avalidator, err := attestation.NewValidator(outm, cfg.ValidatorRegistryLimit, bc.GetBlockReward())
	if err != nil {
		return nil, err
	}

	// new block miner
	forger := miner.NewMiner(bc, validator, txPool, outm, &miner.MinerConfig{
		ShowStat:     statFlag,
		ShowFullStat: expStatFlag,
	})

	ctx, finish := context.WithCancel(context.Background())

	srv := &Service{
		cliCtx:     cliCtx,
		ctx:        ctx,
		cancelFunc: finish,
		outm:       outm,
		bc:         bc,
		miner:      forger,
		txPool:     txPool,

		stop: make(chan struct{}),

		// flags
		fullStatFlag: statFlag,
		expStatFlag:  expStatFlag,

		ready: false,
	}

	return srv, nil
}

// Service implements blockchain service for blockchain update, read and creating new blocks.
type Service struct {
	cliCtx       *cli.Context
	ctx          context.Context
	cancelFunc   context.CancelFunc
	statusErr    error
	startFailure error
	stop         chan struct{}

	outm  *OutputManager // output manager
	bc    *BlockChain    // blockchain
	miner *miner.Miner   // block miner

	txPool *txpool.TxPool // TODO remove it to another service

	// flags
	fullStatFlag bool
	expStatFlag  bool

	mu sync.RWMutex

	blockStat map[string]int64

	ready bool
}

// Start service work
func (s *Service) Start() {
	log.Warn("Start Chain service.")

	// sync database
	err := s.SyncDatabase()
	if err != nil {
		log.Errorf("SyncDatabase fail: %s", err)

		s.mu.Lock()
		s.startFailure = err
		s.mu.Unlock()
		return
	}

	// start block generator main loop
	go s.generatorLoop()
}

// generatorLoop is main loop of service
func (s *Service) generatorLoop() {
	log.Warn("[ChainService] Start block miner loop.")

	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()

	blockTicker := time.NewTicker(generatorInterval)
	defer blockTicker.Stop()

	for {
		select {
		case <-s.stop:
			return
		case <-blockTicker.C:
			start := time.Now()

			num, err := s.genBlockWorker()
			if err != nil {
				log.Errorf("[ChainService] generatorLoop: Error: %s", err.Error())

				s.mu.Lock()
				s.statusErr = err
				s.mu.Unlock()
				return
			}

			if s.fullStatFlag {
				end := time.Since(start)
				log.Infof("[ChainService] Create block in: %s", common.StatFmt(end))
			}

			log.Warnf("[ChainService] Block #%d generated.", num)
		}
	}
}

// genBlockWorker worker for creating one block and store it to the database
func (s *Service) genBlockWorker() (uint64, error) {
	start := time.Now()

	// generate block with block miner
	block, err := s.miner.GenerateBlock()
	if err != nil {
		return 0, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("[ChainService] genBlockWorker: Generate block in %s.", common.StatFmt(end))
	}

	start = time.Now()

	// validate, save block and update SQL
	err = s.miner.FinalizeBlock(block)
	if err != nil {
		log.Error("[ChainService] genBlockWorker: Error finalizing block.")

		return 0, err
	}

	if s.fullStatFlag {
		end := time.Since(start)
		log.Infof("[ChainService] genBlockWorker: Finalize block in %s.", common.StatFmt(end))
	}

	return block.Num, nil
}

// Stop stops tx generator service
func (s *Service) Stop() error {
	close(s.stop)  // close stop chan
	s.cancelFunc() // finish context

	s.showEndStats()

	log.Warn("Stop service.")
	return nil
}

// showEndStats write stats when stop service
func (s *Service) showEndStats() {
	log.Printf("[ChainService] Blockhain has %d blocks.", s.bc.GetCurrentBlockNum())
}

func (s *Service) Status() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.startFailure != nil {
		return s.startFailure
	}

	return s.statusErr
}

// SyncDatabase sync SQL with KV and check that total amount is equal to initial amount.
// If amounts are equal system is working correctly.
func (s *Service) SyncDatabase() error {
	log.Warn("Start database syncing.")

	// sync database data
	err := s.outm.SyncData()
	if err != nil {
		return err
	}

	// check DB consistency
	err = s.outm.CheckBalance()
	if err != nil {
		return err
	}

	return nil
}


// FindAllUTxO returns all address unspent outputs.
func (s *Service) FindAllUTxO(addr string) ([]*types.UTxO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.outm.FindAllUTxO(addr)
}

// GetSyncStatus return sync status of local blockchain with network
func (s *Service) GetSyncStatus() (string, error) {
	s.mu.RLock()
	isNodeReady := s.ready
	s.mu.RUnlock()

	statusMsg := "Not ready."
	if isNodeReady {
		statusMsg = "Ready. Synced."
	}

	return statusMsg, nil
}

func (s *Service) GetServiceStatus() (string, error) {
	return s.GetSyncStatus()
}

// SendRawTx implements PoolAPI for gRPC gateway // TODO remove it to another service
func (s *Service) SendRawTx(tx *prototype.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.txPool.SendTx(tx)
	if err != nil {
		return status.Error(17, err.Error())
	}

	return nil
}


func (s *Service) GetBlockByNum(n uint64) (*prototype.Block, error) {
	return s.bc.GetBlockByNum(n)
}

func (s *Service) GetBlockByHash(hexHash string) (*prototype.Block, error) {
	hash := common.HexToHash(hexHash)
	return s.bc.GetBlockByHash(hash.Bytes())
}

func (s *Service) GetBalance(addr string) (uint64, error) {
	utxo, err := s.FindAllUTxO(addr)
	if err != nil {
		return 0, err
	}

	var balance uint64 = 0
	for _, uo := range utxo {
		balance += uo.Amount
	}

	return balance, nil
}

func (s *Service) GetTransaction(hash string) (*prototype.Transaction, error) {
	return s.bc.GetTransaction(hash)
}

// GetStakeDeposits returns all address stake deposits.
func (s *Service) GetStakeDeposits(addr string) ([]*types.UTxO, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.outm.FindStakeDepositsOfAddress(addr)
}