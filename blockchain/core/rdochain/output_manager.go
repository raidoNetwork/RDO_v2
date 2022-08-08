package rdochain

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/utils/async"
	"github.com/raidoNetwork/RDO_v2/utils/serialize"
	"math"
	"strings"
	"sync"
	"time"
)

var (
	ErrAffectedRows = errors.New("Rows affected are different from expected")
)

const blocksPerTx = 10

func NewOutputManager(bc *BlockChain, outDB db.OutputStorage) *OutputManager {
	om := OutputManager{
		db:          outDB,
		bc:          bc,
		minBlockNum: 0,
		maxBlockNum: 0,
	}

	return &om
}

// OutputManager control all outputs data and sync it with blockchain.
type OutputManager struct {
	db db.OutputStorage
	bc *BlockChain

	// sync status data
	minBlockNum uint64
	maxBlockNum uint64

	mu sync.Mutex
	finalizeLock async.Mutex

	syncing bool
}

// FindAllUTxO find all address' unspent outputs
func (om *OutputManager) FindAllUTxO(from string) ([]*types.UTxO, error) {
	om.finalizeLock.WaitLock()
	return om.db.FindAllUTxO(from)
}

// ProcessBlock update SQL data according to changes in the given block.
func (om *OutputManager) ProcessBlock(block *prototype.Block) error {
	start := time.Now()

	blockTx, err := om.db.CreateTx(true)
	if err != nil {
		log.Errorf("OutputManager.processBlock: Error creating DB tx. %s.", err)
		return err
	}

	var from common.Address
	var startInner time.Time
	var arows int64

	var queryPart string // query data of one transaction
	var outputsCount int // outputs count of transaction
	var txHash common.Hash

	blockOutputs := 0 // count of all block outputs

	var queryBuilder strings.Builder

	// update SQL
	for _, tx := range block.Transactions {
		// update outputs section
		startInner = time.Now()

		txHash = common.BytesToHash(tx.Hash)
		if common.HasInputs(tx) {
			from = common.BytesToAddress(tx.Inputs[0].Address)

			// update tx inputs in database
			err = om.processBlockInputs(blockTx, tx)
			if err != nil {
				log.Errorf("Error processing block inputs on tx %s: %s", txHash, err)
				return err
			}
		}

		// create tx outputs in the database
		queryPart, outputsCount = om.prepareOutputsQuery(tx, from, block.Num)

		// add divide coma
		if queryBuilder.Len() > 0 && queryPart != "" {
			queryBuilder.WriteString(", ")
		}

		queryBuilder.WriteString(queryPart)
		blockOutputs += outputsCount
		updateTxDataTime.Observe(float64(time.Since(startInner).Milliseconds()))
	}

	query := queryBuilder.String()

	// add all outputs batch
	if query != "" {
		arows, err = om.db.AddOutputBatch(blockTx, query)
		if err != nil || arows != int64(blockOutputs) {
			log.Error("Error inserting outputs batch.")
			return om.processUpdateError(arows, int64(blockOutputs), blockTx, err)
		}
	}

	err = om.db.CommitTx(blockTx)
	if err != nil {
		log.Error("OutputManager.processBlock: Error committing block transaction.")
		return om.rollbackAndGetError(blockTx, err)
	}

	storeTransactionsTime.Observe(float64(time.Since(start).Milliseconds()))

	return nil
}

// SyncData synchronize SQL data with KV.
func (om *OutputManager) SyncData() error {
	lastSQLBlockNum, err := om.db.FindLastBlockNum()
	if err != nil {
		return err
	}

	lastKVBlockNum := om.bc.GetHeadBlockNum()

	log.Infof("Current block in KV %d - SQL %d.", lastKVBlockNum, lastSQLBlockNum)

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

		localDbSyncTime.Observe(float64(time.Since(start).Milliseconds()))
	}

	return nil
}

// syncBlock sync SQL data with given block
func (om *OutputManager) syncBlock(block *prototype.Block, blockTx int) error {
	start := time.Now()

	if block.Num == GenesisBlockNum {
		err := om.cleanGenesisUTxO(blockTx)
		if err != nil {
			return err
		}
	}

	var index uint32
	var from []byte
	var hash common.Hash

	for _, tx := range block.Transactions {
		for _, in := range tx.Inputs {
			hash = common.BytesToHash(in.Hash)

			// no need to check affected rows
			// because it is needed to remove them all
			_, err := om.db.SpendOutput(blockTx, hash.Hex(), in.Index)
			if err != nil {
				return om.rollbackAndGetError(blockTx, err)
			}
		}

		if common.IsLegacyTx(tx) {
			from = tx.Inputs[0].Address
		} else {
			from = nil
		}

		index = 0
		for _, out := range tx.Outputs {
			// skip all fee outputs
			if tx.Type == common.FeeTxType && common.BytesToAddress(out.Address).Hex() == common.BlackHoleAddress {
				continue
			}

			uo := types.NewUTxO(tx.Hash, from, out.Address, out.Node, index, out.Amount, block.Num, tx.Type, tx.Timestamp)
			err := om.db.AddOutputIfNotExists(blockTx, uo)
			if err != nil {
				return om.rollbackAndGetError(blockTx, err)
			}

			index++
		}
	}

	localDbSyncBlockTime.Observe(float64(time.Since(start).Milliseconds()))

	return nil
}

// syncLastBlock sync last block from KV with SQL
func (om *OutputManager) syncLastBlock() error {
	block, err := om.bc.GetHeadBlock()
	if err != nil {
		return err
	}

	tx, err := om.db.CreateTx(false)
	if err != nil {
		return err
	}

	if block.Num == GenesisBlockNum {
		err := om.cleanGenesisUTxO(tx)
		if err != nil {
			return err
		}
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
	for blockNum := min; blockNum <= max; blockNum++ {
		if counter%blocksPerTx == 0 {
			// commit tx
			if blockNum != min {
				err = om.db.CommitTx(tx)
				if err != nil {
					return err
				}
			}

			// create new tx
			tx, err = om.db.CreateTx(false)
			if err != nil {
				return err
			}
		}

		if clear && blockNum != min {
			err = om.db.DeleteOutputs(tx, blockNum)
			if err != nil {
				return err
			}
		} else {
			block, err = om.bc.GetBlockByNum(blockNum)
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

		log.Debugf("Sync block %d / %d", blockNum, max)
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
	var start time.Time
	txHash := common.Encode(tx.Hash)

	// update inputs
	for _, in := range tx.Inputs {
		start = time.Now()
		hash := common.BytesToHash(in.Hash).Hex()

		arows, err := om.db.SpendOutput(blockTx, hash, in.Index)
		if err != nil || arows != 1 {
			log.WithError(err).Errorf("Error deleting input: %s_%d.", hash, in.Index)
			return om.processUpdateError(arows, 1, blockTx, err)
		}

		log.Debugf("Delete input %s on %s", serialize.GenKeyFromPbInput(in), txHash)
		inputsSavingTime.Observe(float64(time.Since(start).Milliseconds()))
	}

	return nil
}

// prepareOutputsQuery creates all transaction outputs query for db
func (om *OutputManager) prepareOutputsQuery(tx *prototype.Transaction, from common.Address, blockNum uint64) (string, int) {
	var uo *types.UTxO
	counter := 0

	var queryBuilder strings.Builder
	var index uint32
	for _, out := range tx.Outputs {
		// skip all fee outputs
		if tx.Type == common.FeeTxType && common.BytesToAddress(out.Address).Hex() == common.BlackHoleAddress {
			index++
			continue
		}

		uo = types.NewUTxO(tx.Hash, from.Bytes(), out.Address, out.Node, index, out.Amount, blockNum, tx.Type, tx.Timestamp)

		if counter > 0 {
			queryBuilder.WriteString(", ")
		}

		queryBuilder.WriteString(uo.ToInsertQuery())
		counter++
		index++
	}

	return queryBuilder.String(), counter
}

// GetSyncStatus return current block num, max block num and percent of synchronisation.
func (om *OutputManager) GetSyncStatus() (uint64, uint64, float64) {
	om.mu.Lock()
	defer om.mu.Unlock()

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
	om.finalizeLock.WaitLock()
	return om.db.FindStakeDepositsOfAddress(address)
}

// IsSyncing return true if OutputManager is syncing with KV.
func (om *OutputManager) IsSyncing() bool {
	om.mu.Lock()
	defer om.mu.Unlock()

	return om.syncing
}

func (om *OutputManager) GetTotalAmount() (uint64, error) {
	return om.db.GetTotalAmount()
}

func (om *OutputManager) cleanGenesisUTxO(tx int) error {
	err := om.db.DeleteOutputs(tx, GenesisBlockNum)
	if err != nil {
		errb := om.db.RollbackTx(tx)
		if errb != nil {
			log.Errorf("Clearing Genesis: %s", errb)
		}

		return err
	}

	return nil
}

func (om *OutputManager) FinalizeLock() {
	om.finalizeLock.Lock()
}

func (om *OutputManager) WaitFinalizeLock() {
	om.finalizeLock.WaitLock()
}

func (om *OutputManager) FinalizeUnlock() {
	om.finalizeLock.Unlock()
}

