package miner

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/validator"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/txpool"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "Miner")

// MinerConfig miner options
type MinerConfig struct {
	ShowStat     bool
	ShowFullStat bool
	BlockSize    int
}

func NewMiner(bc consensus.BlockMiner, v consensus.BlockValidator, av consensus.StakeValidator, txPool *txpool.TxPool, outm consensus.OutputUpdater, cfg *MinerConfig) *Miner {
	m := Miner{
		bc:                   bc,
		validator:            v,
		attestationValidator: av,
		txPool:               txPool,
		outm:                 outm,
		cfg:                  cfg,
	}

	return &m
}

type Miner struct {
	bc   consensus.BlockMiner
	outm consensus.OutputUpdater

	validator            consensus.BlockValidator
	attestationValidator consensus.StakeValidator
	txPool               *txpool.TxPool

	cfg  *MinerConfig
	lock sync.RWMutex
}

// GenerateBlock create block from tx pool data
func (m *Miner) GenerateBlock() (block *prototype.Block, err error) {
	block, err = m.generateBlockWorker() // try to generate block

	if err != nil {
		log.Errorf("GenerateBlock: %s.", err)
		return
	}

	return
}

// generateBlockWorker tries to create block with given BlockSize limit.
func (m *Miner) generateBlockWorker() (*prototype.Block, error) {
	totalSize := 0 // current size of block in bytes

	txList := m.txPool.GetPricedQueue()
	txListLen := len(txList)
	txBatch := make([]*prototype.Transaction, 0)

	// create reward transaction for current block
	rewardTx, err := m.attestationValidator.CreateRewardTx(m.bc.GetBlockCount())
	if err != nil {
		if errors.Is(err, validator.ErrNoStakers) {
			log.Warn("No stakers on current block.")
		} else {
			return nil, err
		}
	} else {
		txBatch = append(txBatch, rewardTx)
		totalSize += rewardTx.SizeSSZ()
	}

	stakeCounter := 0

	var size int
	for i := 0; i < txListLen; i++ {
		// skip already checked transactions
		if txList[i].IsChecked() {
			continue
		}

		size = txList[i].Size()
		totalSize += size

		// mark tx as checked
		txList[i].SetChecked()

		if totalSize <= m.cfg.BlockSize {
			tx := txList[i].GetTx()

			// check that stake slots are open
			if tx.Type == common.StakeTxType && !m.attestationValidator.CanStake() {
				log.Warnf("Skip stake transaction %s because all slots are filled.", common.BytesToHash(tx.Hash))

				// rollback changes
				txList[i].UnsetChecked()
				totalSize -= size

				stakeCounter++

				continue
			}

			txBatch = append(txBatch, tx)

			// we fill block successfully
			if totalSize == m.cfg.BlockSize {
				break
			}

			// try to add in the block stake transactions skipped above
			if tx.Type == common.UnstakeTxType && stakeCounter > 0 {
				i = 0
			}
		} else {
			// tx is too big try for look up another one
			totalSize -= size
		}
	}

	// set given tx as received in order to delete them from pool
	if txListLen > 0 {
		err = m.txPool.ReserveTransactions(txBatch)
		if err != nil {
			return nil, err
		}

		log.Warnf("Generate block with transactions count: %d. TxPool transactions count: %d. Size: %d kB.", len(txBatch), txListLen, totalSize/1024)
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

	for i := 0; i < len(block.Transactions); i++ {
		tx := block.Transactions[i]

		if tx.Type == common.StakeTxType {
			err = m.attestationValidator.RegisterStake(tx.Inputs[0].Address)
			if err != nil {
				log.Errorf("Wrong block created!!! Error: %s", err)
				return err
			}
		}

		if tx.Type == common.UnstakeTxType {
			err = m.attestationValidator.UnregisterStake(tx.Inputs[0].Address)
			if err != nil {
				log.Errorf("Undefined staker! Error: %s.", err)
				return err
			}
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
