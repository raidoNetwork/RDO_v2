package miner

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
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

func NewMiner(bc consensus.BlockMiner, v consensus.BlockValidator, av consensus.StakeValidator, txPool consensus.TransactionQueue, outm consensus.OutputUpdater, cfg *MinerConfig) *Miner {
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
	txPool               consensus.TransactionQueue

	cfg  *MinerConfig
	lock sync.RWMutex
}

// GenerateBlock create block from tx pool data
func (m *Miner) GenerateBlock() (*prototype.Block, error) {
	totalSize := 0 // current size of block in bytes

	txList := m.txPool.GetTxQueue()
	txListLen := len(txList)
	txBatch := make([]*prototype.Transaction, 0, txListLen)

	// create reward transaction for current block
	rewardTx, err := m.attestationValidator.CreateRewardTx(m.bc.GetBlockCount())
	if err != nil {
		if errors.Is(err, consensus.ErrNoStakers) {
			log.Warn("No stakers on current block.")
		} else {
			return nil, err
		}
	} else {
		txBatch = append(txBatch, rewardTx)
		totalSize += rewardTx.SizeSSZ()
	}

	// limit tx count in block according to marshaller settings
	txBatchLimit := 500
	if txListLen < txBatchLimit {
		txBatchLimit = txListLen
	}

	var size int
	for i := 0; i < txBatchLimit; i++ {
		size = txList[i].Size()
		totalSize += size

		if totalSize <= m.cfg.BlockSize {
			tx := txList[i].GetTx()

			// check if empty validator slots exists and skip stake tx if not exist
			if tx.Type == common.StakeTxType {
				err = m.attestationValidator.ReserveSlot()
				if err != nil {
					totalSize -= size // return size of current tx

					log.Infof("Skip tx %s", common.Encode(tx.Hash))

					// Delete stake tx from pool
					err = m.txPool.DeleteTransaction(tx)
					if err != nil {
						return nil, errors.Wrap(err, "Error creating block")
					}

					continue
				}
			}

			txBatch = append(txBatch, tx)

			// we fill block successfully
			if totalSize == m.cfg.BlockSize {
				break
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
	}

	// get block instance
	block, err := m.bc.GenerateBlock(txBatch)
	if err != nil {
		return nil, err
	}

	log.Warnf("Generate block with transactions count: %d. TxPool transactions count: %d. Size: %d kB.", len(txBatch), txListLen, totalSize/1024)

	return block, nil
}

// FinalizeBlock validate given block and save it to the blockchain
func (m *Miner) FinalizeBlock(block *prototype.Block) error {
	start := time.Now()

	// validate block
	err := m.validator.ValidateBlock(block)
	if err != nil {
		m.txPool.RollbackReserved()
		return errors.Wrap(err, "ValidateBlockError")
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
		log.Errorf("FinalizeBlock: Error process block: %s", err)

		// try to resync SQL with KV
		err = m.outm.SyncData()
		if err != nil {
			return err
		}
	}

	// update stake slots
	for _, tx := range block.Transactions {
		if tx.Type == common.StakeTxType {
			err = m.attestationValidator.RegisterStake(tx.Inputs[0].Address)
			if err != nil {
				log.Errorf("Error proccessing stake transaction: %s", err)
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

	// Reset reserved validator slots
	m.attestationValidator.FlushReserved()

	// Reset reserved pool and clean all extra staking
	m.txPool.FlushReserved(true)

	if m.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("FinalizeBlock: Proccess block in %s", common.StatFmt(end))
	}

	return nil
}
