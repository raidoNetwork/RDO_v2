package rdochain

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/blockchain/state"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"sync"
)

var log = logrus.WithField("prefix", "blockchain")

func NewService(kv db.BlockStorage, sql db.OutputStorage, stateFeed events.Feed) (*Service, error){
	cfg := params.RaidoConfig()

	// create blockchain instance
	bc := NewBlockChain(kv, cfg)

	// output manager
	outm := NewOutputManager(bc, sql)

	srv := &Service{
		bc: bc,
		outm: outm,
		stateFeed: stateFeed,
	}

	return srv, nil
}

type Service struct{
	bc *BlockChain
	outm *OutputManager
	stateFeed events.Feed
	mu sync.Mutex
	ready bool
	statusErr error
	startFailure error
}

func (s *Service) Start(){
	// load head data and Genesis
	err := s.bc.Init()
	if err != nil {
		log.Errorf("Fail blockchain start: %s", err)

		s.mu.Lock()
		s.startFailure = err
		s.mu.Unlock()
		return
	}

	// sync database
	err = s.SyncDatabase()
	if err != nil {
		log.Errorf("Outputs sync fail: %s", err)

		s.mu.Lock()
		s.startFailure = err
		s.mu.Unlock()
		return
	}

	// change service status
	s.mu.Lock()
	s.ready = true
	s.mu.Unlock()

	s.stateFeed.Send(state.LocalSynced)
}

func (s *Service) Status() error {
	return nil
}

func (s *Service) Stop() error {
	return nil
}

// SyncDatabase sync SQL with KV.
func (s *Service) SyncDatabase() error {
	// sync database data
	err := s.outm.SyncData()
	if err != nil {
		return err
	}

	err = s.CheckBalance()
	if err != nil {
		return err
	}

	return nil
}

// CheckBalance check that total supply of chain is correct
func (s *Service) CheckBalance() error {
	// get amount stats from KV
	rewardAmount, feeAmount, genesisSupply := s.bc.GetAmountStats()

	// get current balances sum from SQL
	balanceSum, err := s.outm.GetTotalAmount()
	if err != nil {
		return err
	}

	targetSum := genesisSupply + rewardAmount
	currentSum := balanceSum + feeAmount

	updateBalanceMetrics(rewardAmount, feeAmount, balanceSum)
	log.Debugf("Genesis: %d Rewards: %d Balances: %d Fees: %d", genesisSupply, rewardAmount, balanceSum, feeAmount)

	if targetSum != currentSum {
		return errors.Errorf("Wrong total supply. Expected: %d. Given: %d.", targetSum, currentSum)
	}

	log.Warnf("System balance is correct. Total supply: %d roi", currentSum)

	return nil
}

// FindAllUTxO returns all address unspent outputs.
func (s *Service) FindAllUTxO(addr string) ([]*types.UTxO, error) {
	return s.outm.FindAllUTxO(addr)
}

// GetSyncStatus return sync status of local blockchain with network
func (s *Service) GetSyncStatus() (string, error) {
	return s.getSQLsyncStatus()
}

func (s *Service) getSQLsyncStatus() (string, error){
	s.mu.Lock()
	isNodeReady := s.ready
	statusError := s.statusErr
	s.mu.Unlock()

	statMsg := ""

	if statusError == nil {
		min, max, percent := s.outm.GetSyncStatus()
		statMsg = fmt.Sprintf("Local sync: blocks %d / %d (%.2f%%)", min, max, percent)
	} else {
		statMsg = "Bye..."
	}

	statusMsg := "Not ready. " + statMsg
	if isNodeReady && !s.outm.IsSyncing() {
		statusMsg = "Ready"
	}

	return statusMsg, nil
}

func (s *Service) GetServiceStatus() (string, error) {
	return s.GetSyncStatus()
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


func (s *Service) GetBlockByNum(n uint64) (*prototype.Block, error) {
	return s.bc.GetBlockByNum(n)
}

func (s *Service) GetBlockByHashHex(hexHash string) (*prototype.Block, error) {
	hash := common.HexToHash(hexHash)
	return s.bc.GetBlockByHash(hash.Bytes())
}

func (s *Service) GetBlockByHash(hash []byte) (*prototype.Block, error){
	return s.bc.GetBlockByHash(hash)
}

func (s *Service) GetTransaction(hash string) (*prototype.Transaction, error) {
	return s.bc.GetTransaction(hash)
}

// GetStakeDeposits returns all address stake deposits.
func (s *Service) GetStakeDeposits(addr string) ([]*types.UTxO, error) {
	return s.outm.FindStakeDepositsOfAddress(addr)
}

func (s *Service) GetTransactionsCountHex(addr string) (uint64, error) {
	return s.bc.GetTransactionsCount(common.HexToAddress(addr).Bytes())
}

func (s *Service) GetTransactionsCount(addr []byte) (uint64, error) {
	return s.bc.GetTransactionsCount(addr)
}

// GetLatestBlock returns the head block of blockchain
func (s *Service) GetLatestBlock() (*prototype.Block, error) {
	return s.bc.GetHeadBlock()
}

// FindStakeDeposits find all stake slots
func (s *Service) FindStakeDeposits() ([]*types.UTxO, error) {
	return s.outm.FindStakeDeposits()
}

// FindStakeDepositsOfAddress return list of stake deposits actual to the moment of block with given num.
func (s *Service) FindStakeDepositsOfAddress(address string) ([]*types.UTxO, error) {
	return s.outm.FindStakeDepositsOfAddress(address)
}

func (s *Service) GetBlockCount() uint64 {
	return s.bc.GetBlockCount()
}

func (s *Service) ParentHash() []byte{
	return s.bc.ParentHash()
}

func (s *Service) SyncData() error {
	return s.outm.SyncData()
}

func (s *Service) GetGenesis() *prototype.Block {
	return s.bc.GetGenesis()
}

// FinalizeBlock save block to the local databases
func (s *Service) FinalizeBlock(block *prototype.Block) error {
	// save block
	err := s.bc.SaveBlock(block)
	if err != nil {
		return errors.Wrap(err, "KV error")
	}

	// update SQL
	err = s.outm.ProcessBlock(block)
	if err != nil {
		return errors.Wrap(err, "SQL error")
	}

	return nil
}