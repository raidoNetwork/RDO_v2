package miner

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/txpool"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/hashutil"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

const (
	BlockSize       = 10 * 1024 // 10 Kb
	TxLimitPerBlock = 5
)

var log = logrus.WithField("prefix", "Miner")

// MinerConfig miner options
type MinerConfig struct {
	ShowStat     bool
	ShowFullStat bool
}

func NewMiner(bc BlockMiner, v consensus.BlockValidator, txPool *txpool.TxPool, outm OutputUpdater, cfg *MinerConfig) *Miner {
	m := Miner{
		bc:        bc,
		validator: v,
		txPool:    txPool,
		outm:      outm,
		cfg:       cfg,
	}

	return &m
}

type Miner struct {
	bc   BlockMiner
	outm OutputUpdater

	validator consensus.BlockValidator
	txPool    *txpool.TxPool

	cfg  *MinerConfig
	lock sync.RWMutex
}

// GenerateBlock create block from tx pool data
func (m *Miner) GenerateBlock() (block *prototype.Block, err error) {
	triesCount := 2

	for i := 0; i < triesCount; i++ {
		block, err = m.generateBlockWorker() // try to generate block

		if err == nil {
			// no errors means that block was mined succesfully

			// check that block is not empty
			if len(block.Transactions) < 2+TxLimitPerBlock {
				return nil, errors.Errorf("Tx pool is empty. Expected %d. Given %d.", 2+TxLimitPerBlock, len(block.Transactions))
			}

			return
		} else {
			log.Errorf("GenerateBlock: %s.", err)

			isNotCriticalErr := errors.Is(err, hashutil.ErrMerkleeTreeCreation) || errors.Is(err, txpool.ErrTxNotFound)
			if isNotCriticalErr {
				continue
			} else {
				return
			}
		}
	}

	return nil, errors.New("All attempts to create block was unsuccessful.")
}

// generateBlockWorker tries to create block with given BlockSize limit.
func (m *Miner) generateBlockWorker() (*prototype.Block, error) {
	totalSize := 0 // current size of block in bytes

	txList := m.txPool.GetPricedQueue()
	txListLen := len(txList)
	txBatch := make([]*prototype.Transaction, 0)

	if txListLen < TxLimitPerBlock {
		log.Infof("Waiting for new transactions in pool.")
	}

	// TODO change wait algorythm
	// lock main thread and wait till pool add needed transaction count
	for txListLen < TxLimitPerBlock {
		txList = m.txPool.GetPricedQueue()
		txListLen = len(txList)
	}

	var size int
	for i := 0; i < txListLen; i++ {
		size = txList[i].Size()
		totalSize += size

		if totalSize <= BlockSize {
			txBatch = append(txBatch, txList[i].GetTx())

			// we fill block successfully
			if totalSize == BlockSize {
				break
			}
		} else {
			// tx is too big try for look up another one
			totalSize -= size
		}
	}

	// set given tx as received in order to delete them from pool
	err := m.txPool.ReserveTransactions(txBatch)
	if err != nil {
		return nil, err
	}

	// get block instance
	block, err := m.bc.GenerateBlock(txBatch)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// FinalizeBlock validate given block and save it to the blockchain
func (m *Miner) FinalizeBlock(block *prototype.Block) error {
	start := time.Now()

	// on error return transactions to the pool

	// validate block
	err := m.validator.ValidateBlock(block)
	if err != nil {
		m.txPool.RollbackReserved()
		return err
	}

	if m.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("FinalizeBlock: Validate block in %s", common.StatFmt(end))
	}

	start = time.Now()

	// save block
	err = m.bc.SaveBlock(block)
	if err != nil {
		m.txPool.RollbackReserved()
		return err
	}

	if m.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("FinalizeBlock: Store block in %s", common.StatFmt(end))
	}

	start = time.Now()

	// update SQL
	err = m.outm.ProcessBlock(block)
	if err != nil {
		log.Errorf("FinalizeBlock: Error process block: %s.", err)

		// try to resync SQL with KV
		err = m.outm.SyncData()
		if err != nil {
			return err
		}
	}

	// Reset reserved pool
	m.txPool.FlushReserved(true)

	if m.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("FinalizeBlock: Proccess block in %s", common.StatFmt(end))
	}

	return nil
}
