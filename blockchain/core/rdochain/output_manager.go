package rdochain

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"math"
	"sync"
	"time"
)

var (
	ErrAffectedRows = errors.New("Rows affected are different from expected")
)

type OutputManagerConfig struct {
	ShowStat     bool
	ShowWideStat bool
}

func NewOutputManager(bc *BlockChain, outDB db.OutputStorage, cfg *OutputManagerConfig) *OutputManager {
	om := OutputManager{
		db:          outDB,
		bc:          bc,
		cfg:         cfg,
		mu:          sync.RWMutex{},
		minBlockNum: 0,
		maxBlockNum: 0,
	}

	return &om
}

// OutputManager control all outputs data and sync it with blockchain.
type OutputManager struct {
	db db.OutputStorage
	bc *BlockChain

	cfg     *OutputManagerConfig
	syncing bool

	// sync status data
	minBlockNum uint64
	maxBlockNum uint64

	mu sync.RWMutex
}

// FindAllUTxO find all address' unspent outputs
func (om *OutputManager) FindAllUTxO(from string) ([]*types.UTxO, error) {
	return om.db.FindAllUTxO(from)
}

// ProcessBlock update SQL data according to changes in the given block.
func (om *OutputManager) ProcessBlock(block *prototype.Block) error {
	start := time.Now()

	blockTx, err := om.db.CreateTx()
	if err != nil {
		log.Errorf("OutputManager.processBlock: Error creating DB tx. %s.", err)
		return err
	}

	var from common.Address
	var startTxInner, startInner time.Time
	var endTxInner, endInner time.Duration
	var arows int64

	var queryPart string // query data of one transaction
	var outputsCount int // outputs count of transaction
	var txHash common.Hash

	query := ""
	blockOutputs := 0 // count of all block outputs

	// update SQL
	for _, tx := range block.Transactions {
		txHash = common.BytesToHash(tx.Hash)

		startTxInner = time.Now() // whole transaction process time
		startInner = time.Now()   // tx inputs process time

		// Only legacy tx has inputs
		if common.IsLegacyTx(tx) {
			from = common.BytesToAddress(tx.Inputs[0].Address)

			// update tx inputs in database
			err = om.processBlockInputs(blockTx, tx)
			if err != nil {
				log.Error("Error processing block inputs on tx", txHash, ":", err)
				return err
			}
		}

		if om.cfg.ShowWideStat {
			endInner = time.Since(startInner)
			log.Infof("OutputManager.processBlock: Update tx %s inputs in %s.", txHash, common.StatFmt(endInner))
		}

		// update outputs section
		startInner = time.Now()

		// create tx outputs in the database
		queryPart, outputsCount = om.prepareOutputsQuery(tx, from, block.Num)

		// add divide coma
		if query != "" && queryPart != "" {
			query += ", "
		}

		// update query
		query += queryPart

		// update outputs counter
		blockOutputs += outputsCount

		if om.cfg.ShowWideStat {
			endInner = time.Since(startInner)
			log.Infof("OutputManager.processBlock: Prepare tx %s outputs query in %s.", txHash, common.StatFmt(endInner))
		}

		if om.cfg.ShowStat {
			endTxInner = time.Since(startTxInner)
			log.Infof("OutputManager.processBlock: Update tx %s data in %s.", txHash, common.StatFmt(endTxInner))
		}
	}

	// add all outputs batch
	if query != "" {
		startInner = time.Now()

		arows, err = om.db.AddOutputBatch(blockTx, query)
		if err != nil || arows != int64(blockOutputs) {
			log.Error("Error inserting outputs batch.")
			return om.processUpdateError(arows, int64(blockOutputs), blockTx, err)
		}

		if om.cfg.ShowStat {
			end := time.Since(startInner)
			log.Infof("Update block outputs in %s.", common.StatFmt(end))
		}
	}

	err = om.db.CommitTx(blockTx)
	if err != nil {
		log.Error("OutputManager.processBlock: Error committing block transaction.")
		return om.rollbackAndGetError(blockTx, err)
	}

	if om.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("OutputManager.processBlock: Update all block data in %s.", common.StatFmt(end))
	}

	return nil
}

// SyncData synchronize SQL data with KV.
func (om *OutputManager) SyncData() error {
	lastSQLBlockNum, err := om.db.FindLastBlockNum()
	if err != nil {
		return err
	}

	log.Infof("SQL database max block num %d.", lastSQLBlockNum)

	lastKVBlockNum := om.bc.GetHeadBlockNum()

	om.mu.Lock()
	om.syncing = true
	om.mu.Unlock()

	defer func() {
		om.mu.Lock()
		om.syncing = false
		om.mu.Unlock()
	}()

	if lastSQLBlockNum == lastKVBlockNum {
		err = om.syncLastBlock()
		if err != nil {
			return errors.Wrap(err, "SyncLastBlock error")
		}
	} else {
		start := time.Now()

		om.mu.Lock()

		clear := false
		if lastKVBlockNum > lastSQLBlockNum {
			om.minBlockNum = lastSQLBlockNum
			om.maxBlockNum = lastKVBlockNum
		} else {
			om.minBlockNum = lastKVBlockNum
			om.maxBlockNum = lastSQLBlockNum
			clear = true
		}

		min := om.minBlockNum
		max := om.maxBlockNum

		om.mu.Unlock()

		err = om.syncDataInRange(min, max, clear)
		if err != nil {
			return errors.Wrap(err, "syncDataInRange error")
		}

		if om.cfg.ShowStat {
			end := time.Since(start)
			log.Infof("Sync SQL with KV in %s.", common.StatFmt(end))
		}
	}

	return nil
}

// syncBlock sync SQL data with given block
func (om *OutputManager) syncBlock(block *prototype.Block, blockTx int) error {
	start := time.Now()

	var index uint32
	var from []byte
	var hash common.Hash
	var key string

	for _, tx := range block.Transactions {
		for _, in := range tx.Inputs {
			hash = common.BytesToHash(in.Hash)

			// no need to check affected rows
			// because it is needed to remove them all
			_, err := om.db.SpendOutput(blockTx, hash.Hex(), in.Index)
			if err != nil {
				return om.rollbackAndGetError(blockTx, err)
			}

			log.Debugf("SyncData: Delete input with key %s_%d.", hash, in.Index)
		}

		if common.IsLegacyTx(tx) {
			from = tx.Inputs[0].Address
		} else {
			from = nil
		}

		index = 0 // tx output index

		// get tx hash
		hash = common.BytesToHash(tx.Hash)
		for _, out := range tx.Outputs {
			// skip all fee outputs
			if tx.Type == common.FeeTxType && common.BytesToAddress(out.Address).Hex() == common.BlackHoleAddress {
				continue
			}

			key = hash.Hex() + "_" + fmt.Sprintf("%d", index)
			log.Debugf("SyncData: Insert or updated output %s to the database.", key)

			uo := types.NewUTxO(tx.Hash, from, out.Address, out.Node, index, out.Amount, block.Num, tx.Type, tx.Timestamp)

			err := om.db.AddOutputIfNotExists(blockTx, uo)
			if err != nil {
				return om.rollbackAndGetError(blockTx, err)
			}

			index++
		}
	}

	if om.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("SyncData: Sync block #%d in %s.", block.Num, common.StatFmt(end))
	}

	return nil
}

// syncLastBlock sync last block from KV with SQL
func (om *OutputManager) syncLastBlock() error {
	block, err := om.bc.GetHeadBlock()
	if err != nil {
		return err
	}

	tx, err := om.db.CreateTx()
	if err != nil {
		return err
	}

	err = om.syncBlock(block, tx)
	if err != nil {
		errb := om.db.RollbackTx(tx)
		if errb != nil {
			log.Errorf("SyncData: %s", errb)
		}

		return err
	}

	err = om.db.CommitTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// syncDataInRange sync range of the blocks with SQL.
// If param clear is true all data with blockId in range [min, max] will be removed.
// With false clear param all data with blockId in range [min, max] will be updated.
// cb - callback function
func (om *OutputManager) syncDataInRange(min, max uint64, clear bool) error {
	logMsg := "Start syncing SQL with KV."

	if clear {
		logMsg = "Start clearing SQL to the KV state."
	}

	log.Warn(logMsg)

	var tx int
	var block *prototype.Block
	var err error

	counter := 0
	txIsOpen := false
	for i := min; i <= max; i++ {
		// commit tx each 10 blocks
		if counter%10 == 0 {
			// commit tx
			if i != min {
				txIsOpen = false
				err = om.db.CommitTx(tx)
				if err != nil {
					return err
				}
			}

			// create new tx
			txIsOpen = true
			tx, err = om.db.CreateTx()
			if err != nil {
				return err
			}
		}

		log.Debugf("Counter: %d BlockNum: %d DB tx: %d opened: %v", counter, i, tx, txIsOpen)

		if clear {
			// delete
			err = om.db.DeleteOutputs(tx, i)
			if err != nil {
				return err
			}
		} else {
			block, err = om.bc.GetBlockByNum(i)
			if err != nil {
				return errors.Wrap(err, "GetBlockByNum error: ")
			}

			// sync given block with SQL
			err = om.syncBlock(block, tx)
			if err != nil {
				errb := om.db.RollbackTx(tx)
				if errb != nil {
					log.Errorf("syncBlocks: Rollback error: %s", errb)
				}

				return err
			}
		}

		om.mu.Lock()
		if om.minBlockNum < max {
			om.minBlockNum++
		}
		om.mu.Unlock()

		counter++
	}

	err = om.db.CommitTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// rollbackAndGetError rollback tx and return error
func (om *OutputManager) rollbackAndGetError(txId int, err error) error {
	res := err.Error()

	log.Errorf("OutputManager.errorProcess: %s", res)

	errb := om.db.RollbackTx(txId)
	if errb != nil {
		res += " Rollback error: " + errb.Error()
	}

	return errors.Errorf("%s", res)
}

// processUpdateError
func (om *OutputManager) processUpdateError(arows, targetRows int64, blockTx int, err error) error {
	errb := om.db.RollbackTx(blockTx)
	if errb != nil {
		log.Errorf("OutputManager.processUpdateError: Rollback error on addTx: %s.", errb)
	}

	if err != nil {
		log.Errorf("OutputManager.processUpdateError: %s.", err.Error())
	} else if arows != targetRows {
		errmsg := ""
		if err != nil {
			errmsg = "Error: " + err.Error()
		}

		log.Errorf("OutputManager.processUpdateError: Affected rows error. Got: %d. Expected: %d. %s", arows, targetRows, errmsg)
		return ErrAffectedRows
	}

	return err
}

// processBlockInputs updates all transaction inputs in the database with given DB tx
func (om *OutputManager) processBlockInputs(blockTx int, tx *prototype.Transaction) error {
	var arows int64
	var err error
	var startIn time.Time
	var endIn time.Duration
	var hash string

	// update inputs
	for _, in := range tx.Inputs {
		hash = common.BytesToHash(in.Hash).Hex()

		startIn = time.Now()

		arows, err = om.db.SpendOutput(blockTx, hash, in.Index)
		if err != nil || arows != 1 {
			log.Errorf("Error deleting input: %s_%d.", hash, in.Index)
			return om.processUpdateError(arows, 1, blockTx, err)
		}

		if om.cfg.ShowWideStat {
			endIn = time.Since(startIn)
			log.Infof("OutputManager.processBlockInputs: Spent one output %s_%d in %s", hash, in.Index, common.StatFmt(endIn))
		}
	}

	return nil
}

// prepareOutputsQuery creates all transaction outputs query for db
func (om *OutputManager) prepareOutputsQuery(tx *prototype.Transaction, from common.Address, blockNum uint64) (string, int) {
	var uo *types.UTxO
	counter := 0
	query := ""

	var index uint32
	for _, out := range tx.Outputs {
		// skip all fee outputs
		if tx.Type == common.FeeTxType && common.BytesToAddress(out.Address).Hex() == common.BlackHoleAddress {
			index++
			continue
		}

		uo = types.NewUTxO(tx.Hash, from.Bytes(), out.Address, out.Node, index, out.Amount, blockNum, tx.Type, tx.Timestamp)

		if counter > 0 {
			query += ", "
		}

		query += uo.ToInsertQuery()
		counter++
		index++
	}

	return query, counter
}

// GetSyncStatus return current block num, max block num and percent of synchronisation.
func (om *OutputManager) GetSyncStatus() (uint64, uint64, float64) {
	om.mu.RLock()
	defer om.mu.RUnlock()

	percent := float64(om.minBlockNum) / float64(om.maxBlockNum) * 100

	if math.IsNaN(percent) {
		percent = 0
	}

	return om.minBlockNum, om.maxBlockNum, percent
}

// FindStakeDeposits return list of stake deposits actual to the moment of block with given num.
func (om *OutputManager) FindStakeDeposits() ([]*types.UTxO, error) {
	return om.db.FindStakeDeposits()
}

// FindStakeDepositsOfAddress return list of stake deposits actual to the moment of block with given num.
func (om *OutputManager) FindStakeDepositsOfAddress(address string) ([]*types.UTxO, error) {
	return om.db.FindStakeDepositsOfAddress(address)
}

// IsSyncing return true if OutputManager is syncing with KV.
func (om *OutputManager) IsSyncing() bool {
	om.mu.RLock()
	defer om.mu.RUnlock()

	return om.syncing
}

func (om *OutputManager) GetTotalAmount() (uint64, error) {
	return om.db.GetTotalAmount()
}
