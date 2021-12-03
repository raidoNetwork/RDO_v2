package rdochain

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"strconv"
	"time"
)

var (
	ErrAffectedRows = errors.New("Rows affected are different from expected.")
)

type OutputManagerConfig struct {
	ShowStat     bool
	ShowWideStat bool
}

func NewOutputManager(bc *BlockChain, outDB db.OutputStorage, cfg *OutputManagerConfig) *OutputManager {
	om := OutputManager{
		db:  outDB,
		bc:  bc,
		cfg: cfg,
	}

	return &om
}

// OutputManager control all outputs data and sync it with blockchain.
type OutputManager struct {
	db db.OutputStorage
	bc *BlockChain

	cfg      *OutputManagerConfig
	lastSync int64
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

	// update SQLite
	for _, tx := range block.Transactions {
		txHash := common.BytesToHash(tx.Hash)

		startTxInner := time.Now() // whole transaction
		startInner := time.Now()   // inputs or outputs block

		// Only normal tx has inputs
		if tx.Type == common.NormalTxType {
			from = common.BytesToAddress(tx.Inputs[0].Address)

			// update tx inputs in database
			err := om.processBlockInputs(blockTx, tx)
			if err != nil {
				return err
			}
		} else {
			// for FeeTx and RewardTx no sender
			from = common.BytesToAddress([]byte{})
		}

		if om.cfg.ShowWideStat {
			endInner := time.Since(startInner)
			log.Infof("OutputManager.processBlock: Update tx %s inputs in %s.", txHash, common.StatFmt(endInner))
		}

		// update outputs
		startInner = time.Now()

		// create tx outputs in the database
		err := om.proccessBlockOutputs(blockTx, tx, from.Hex(), block.Num)
		if err != nil {
			return err
		}

		if om.cfg.ShowWideStat {
			endInner := time.Since(startInner)
			log.Infof("OutputManager.processBlock: Update tx %s outputs in %s.", txHash, common.StatFmt(endInner))
		}

		if om.cfg.ShowStat {
			endTxInner := time.Since(startTxInner)
			log.Infof("OutputManager.processBlock: Update tx %s data in %s.", txHash, common.StatFmt(endTxInner))
		}
	}

	err = om.db.CommitTx(blockTx)
	if err != nil {
		log.Error("OutputManager.processBlock: Error committing block transaction.")
		return om.errorProcess(blockTx, err)
	}

	if om.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("Update all block data in %s.", common.StatFmt(end))
	}

	return nil
}

// syncBlock sync SQL data with given block
func (om *OutputManager) syncBlock(block *prototype.Block, blockTx int) error {
	start := time.Now()

	var index uint32
	var from []byte
	for _, tx := range block.Transactions {
		index = 0

		for _, in := range tx.Inputs {
			hash := common.BytesToHash(in.Hash)

			// no need to check affected rows
			// because it is needed to remove them all
			_, err := om.db.SpendOutput(blockTx, hash.Hex(), in.Index)
			if err != nil {
				return om.errorProcess(blockTx, err)
			}

			log.Warnf("SyncData: Delete input with key %s_%d.", hash, in.Index)
		}

		// FIXME for multiple senders
		if tx.Type == common.NormalTxType {
			from = tx.Inputs[0].Address
		} else {
			from = make([]byte, 0)
		}

		// get tx hash
		hash := common.BytesToHash(tx.Hash)
		for _, out := range tx.Outputs {
			key := hash.Hex() + "_" + strconv.FormatUint(uint64(index), 10)

			uo := types.NewUTxO(tx.Hash, from, out.Address, out.Node, index, out.Amount, block.Num, tx.Type, tx.Timestamp)

			arows, err := om.db.VerifyOutput(blockTx, uo)
			if err != nil {
				return om.errorProcess(blockTx, err)
			} else if arows != 1 {
				if arows == 0 {
					log.Warnf("SyncData: Not found output with key %s. Add output to the database.", key)

					addRows, err := om.db.AddOutput(blockTx, uo)
					if err != nil {
						return om.errorProcess(blockTx, err)
					}

					if addRows != 1 {
						return om.errorProcess(blockTx, errors.New("SyncData: Inconsistent SQL database."))
					}
				} else {
					// TODO remove doubles
					return om.errorProcess(blockTx, errors.New("SyncData: Inconsistent SQL database. Check double inputs."))
				}
			}

			index++
		}
	}

	if om.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("SyncData: Sync block #%d in %s.", block.Num, common.StatFmt(end))
	}

	om.lastSync = time.Now().UnixNano()

	return nil
}

// SyncData synchronize SQL data with KV.
func (om *OutputManager) SyncData() error {
	lastSQLBlockNum, err := om.db.FindLastBlockNum()
	if err != nil {
		return err
	}

	log.Infof("SQL database max block num %d.", lastSQLBlockNum)

	lastKVBlockNum := om.bc.GetCurrentBlockNum()

	if lastSQLBlockNum == lastKVBlockNum {
		err = om.syncLastBlock()
		if err != nil {
			return errors.Wrap(err, "SyncLastBlock error: ")
		}
	} else {
		start := time.Now()

		if lastKVBlockNum > lastSQLBlockNum {
			// update SQL according to the KV data
			err = om.syncData(lastSQLBlockNum, lastKVBlockNum, false)
		} else {
			// clear all extra data from SQL
			err = om.syncData(lastKVBlockNum, lastSQLBlockNum, true)
		}

		if err != nil {
			return errors.Wrap(err, "syncData error: ")
		}

		if om.cfg.ShowStat {
			end := time.Since(start)
			log.Infof("Sync SQL with KV in %s.", common.StatFmt(end))
		}
	}

	return nil
}

// syncLastBlock sync last block from KV with SQL
func (om *OutputManager) syncLastBlock() error {
	block, err := om.bc.GetCurrentBlock()
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

// syncData sync range of the blocks with SQL.
// If param clear is true all data with blockId in range [min, max] will be removed.
// With false clear param all data with blockId in range [min, max] will be updated.
// cb - callback function
func (om *OutputManager) syncData(min, max uint64, clear bool) error {
	logMsg := "Start syncing SQL with KV."

	if clear {
		logMsg = "Start clearing SQL to the KV state."
	}

	log.Warn(logMsg)

	var tx int

	counter := 0
	txIsOpen := false // debug flag
	for i := min; i <= max; i++ {
		block, err := om.bc.GetBlockByNum(i)
		if err != nil {
			return errors.Wrap(err, "GetBlockByNum error: ")
		}

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
			if i != max {
				txIsOpen = true
				tx, err = om.db.CreateTx()
				if err != nil {
					return err
				}
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

		counter++
	}

	// finish last tx
	err := om.db.CommitTx(tx)
	if err != nil {
		return err
	}

	return nil
}

// errorProcess rollback tx and return error
func (om *OutputManager) errorProcess(txId int, err error) error {
	res := err.Error()

	log.Errorf("Got error: %s", res)

	errb := om.db.RollbackTx(txId)
	if errb != nil {
		res += " Rollback error: " + errb.Error()
	}

	return errors.Errorf("%s", res)
}

// errorsCheck check error type when updating database and rollback all changes
func (om *OutputManager) errorsCheck(arows int64, blockTx int, err error) error {
	errb := om.db.RollbackTx(blockTx)
	if errb != nil {
		log.Errorf("OutputManager.processBlock: Rollback error on addTx: %s.", errb)
	}

	if arows != 1 {
		errmsg := ""
		if err != nil {
			errmsg = "Error: " + err.Error()
		}

		log.Errorf("OutputManager.processBlock: Affected rows error. Got: %d. %s.", arows, errmsg)
		return ErrAffectedRows
	} else {
		log.Errorf("OutputManager.processBlock: %s.", err.Error())
	}

	return err
}

// processBlockInputs updates all transaction inputs in the database with given DB tx
func (om *OutputManager) processBlockInputs(blockTx int, tx *prototype.Transaction) error {
	var arows int64
	var err error

	// update inputs
	for _, in := range tx.Inputs {
		startIn := time.Now()
		arows, err = om.db.SpendOutput(blockTx, common.BytesToHash(in.Hash).Hex(), in.Index)

		if err != nil || arows != 1 {
			return om.errorsCheck(arows, blockTx, err)
		}

		if om.cfg.ShowWideStat {
			endIn := time.Since(startIn)
			log.Infof("OutputManager.processBlockInputs: Spent one output in %s", common.StatFmt(endIn))
		}
	}

	return nil
}

// processBlockOutputs creates all transaction output in the database with given tx
// and rollback tx if error return
func (om *OutputManager) proccessBlockOutputs(blockTx int, tx *prototype.Transaction, addr string, blockNum uint64) error {
	var index uint32 = 0
	var arows int64
	var err error

	from := common.HexToAddress(addr)

	for _, out := range tx.Outputs {
		startOut := time.Now()

		uo := types.NewUTxO(tx.Hash, from.Bytes(), out.Address, out.Node, index, out.Amount, blockNum, tx.Type, tx.Timestamp)

		// add output to the database
		arows, err = om.db.AddOutput(blockTx, uo)

		if err != nil || arows != 1 {
			return om.errorsCheck(arows, blockTx, err)
		}

		index++

		if om.cfg.ShowWideStat {
			endOut := time.Since(startOut)
			log.Infof("OutputManager.processBlockOutputs: Add one output to DB in %s", common.StatFmt(endOut))
		}
	}

	return nil
}

// CheckBalance check that SQL total amount is equal to the start amount given to the network
func (om *OutputManager) CheckBalance() error {
	sum, err := om.db.GetTotalAmount()
	if err != nil {
		return err
	}

	targetSum := startAmount * common.AccountNum

	if targetSum != sum {
		return errors.Errorf("SQL sync fail. Expect balance: %d. Got: %d.", targetSum, sum)
	}

	log.Warn("SQL sum is correct.")

	return nil
}

// FindStakeDeposits return list of stake deposits actual to the moment of block with given num.
func (om *OutputManager) FindStakeDeposits() ([]*types.UTxO, error) {
	return om.db.FindStakeDeposits()
}

// FindStakeDepositsOfAddress return list of stake deposits actual to the moment of block with given num.
func (om *OutputManager) FindStakeDepositsOfAddress(address string) ([]*types.UTxO, error) {
	return om.db.FindStakeDepositsOfAddress(address)
}
