package txpool

import (
	"bytes"
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrTxExists               = errors.New("tx already exists in pool")
	ErrInputExists            = errors.New("tx input is locked for spend")
	ErrTxNotFound             = errors.New("tx was not found in the pool")
	ErrTxNotFoundInPricedPool = errors.New("tx was not found in priced pool")
	ErrPoolClosed             = errors.New("tx pool is closed for writing")
	ErrAlreadyReserved        = errors.New("tx already reserved")
)

var log = logrus.WithField("prefix", "TxPool")

func NewTxPool(v consensus.TxValidator, cfg *PoolConfig) *TxPool {
	ctx, finish := context.WithCancel(context.Background())

	tp := TxPool{
		validator:      v,
		pool:           map[string]*types.TransactionData{},
		lockedInputs:   map[string]string{},
		pricedPool:     make(pricedTxPool, 0),
		reservedPool:   map[string]*types.TransactionData{},
		villainousPool: map[string]*types.TransactionData{},

		// ctx
		ctx:    ctx,
		finish: finish,
		// channel
		dataC: make(chan *prototype.Transaction),
		cfg:   cfg,
		lock: sync.Mutex{},
	}

	return &tp
}

type PoolConfig struct {
	BlockSize  int
	MinimalFee uint64
}

type TxPool struct {
	lock sync.Mutex

	validator consensus.TxValidator

	// to mark that inputs has been already spent
	// and avoid double spend
	lockedInputs map[string]string

	// valid tx pool map[tx hash] -> tx
	pool map[string]*types.TransactionData

	// priced list
	pricedPool pricedTxPool

	// reserved tx list for future block
	reservedPool map[string]*types.TransactionData

	// double spend tx
	villainousPool map[string]*types.TransactionData

	dataC chan *prototype.Transaction

	ctx    context.Context
	finish context.CancelFunc

	cfg *PoolConfig
}

func (tp *TxPool) Start() {
	log.Warn("Start Attestation service.")

	go tp.ReadingLoop()
}

func (tp *TxPool) Stop() error {
	tp.StopWriting()

	log.Warn("Stop Attestation service.")
	return nil
}

func (tp *TxPool) Status() error {
	return nil
}

// SendRawTx implements PoolAPI for gRPC gateway
func (tp *TxPool) SendRawTx(tx *prototype.Transaction) error {
	err := tp.SendTx(tx)
	if err != nil {
		return status.Error(17, err.Error())
	}

	return nil
}

// SendTx add tx to the pool
func (tp *TxPool) SendTx(tx *prototype.Transaction) error {
	select {
	case <-tp.ctx.Done():
		// got interrupt stop
		close(tp.dataC)
		return ErrPoolClosed
	case tp.dataC <- tx:
		// write to the pool
		return nil
	}
}

// ReadingLoop loop that waits for new transactions and read it
func (tp *TxPool) ReadingLoop() {
	for tx := range tp.dataC {
		err := tp.RegisterTx(tx)
		if err != nil {
			log.Errorf("ReadingLoop: Registration error. %s", err)
		}
	}
}

// RegisterTx validate tx and add it to the pool if it is correct
func (tp *TxPool) RegisterTx(tx *prototype.Transaction) error {
	td := types.NewTxData(tx)

	err := tp.validateTx(td)
	if err != nil {
		return errors.Wrap(err, "TxPool.RegisterTx")
	}

	tp.lock.Lock()
	hash := common.Encode(tx.Hash)

	// save tx to the pool
	tp.pool[hash] = td

	// save tx to the priced pool
	tp.pricedPool = append(tp.pricedPool, td)

	// mark inputs as already spent
	for _, in := range td.GetTx().Inputs {
		key := genKeyFromInput(in)

		// mark the input is spent with tx hash
		tp.lockedInputs[key] = hash

		log.Debugf("Lock input %s for tx %s", key, hash)
	}

	tp.lock.Unlock()

	log.Warnf("Register tx %s in pool.", hash)

	return nil
}

// validateTx validates tx by validator, finds double spends and checks tx exists in the pool
func (tp *TxPool) validateTx(td *types.TransactionData) error {
	txHash := common.Encode(td.GetTx().Hash)

	start := time.Now()

	// validate balance, signatures and hash check
	err := tp.validator.ValidateTransactionStruct(td.GetTx())
	if err != nil {
		return err
	}

	end := time.Since(start)
	log.Infof("Validate tx %s struct in: %s.", txHash, common.StatFmt(end))

	// check tx is in pool already
	tp.lock.Lock()
	_, exists := tp.pool[txHash]
	_, reserved := tp.reservedPool[txHash]
	tp.lock.Unlock()

	if exists || reserved {
		return ErrTxExists
	}

	start = time.Now()

	err = tp.checkInputs(td)
	if err != nil {
		return err
	}

	end = time.Since(start)
	log.Infof("Check tx %s inputs in: %s.", txHash, common.StatFmt(end))

	start = time.Now()

	err = tp.validator.ValidateTransaction(td.GetTx())
	if err != nil {
		return err
	}

	end = time.Since(start)
	log.Infof("Validate tx %s in: %s.", txHash, common.StatFmt(end))

	return nil
}

func (tp *TxPool) checkInputs(td *types.TransactionData) error {
	hash := common.Encode(td.GetTx().Hash)

	tp.lock.Lock()
	defer tp.lock.Unlock()

	// lock inputs
	for _, in := range td.GetTx().Inputs {
		key := genKeyFromInput(in)
		firstTxHash, exists := tp.lockedInputs[key]

		if exists {
			ftd, existsInPool := tp.pool[firstTxHash]

			if existsInPool {
				// If tx has the same sender address reject it.
				if !bytes.Equal(td.GetTx().Inputs[0].Address, ftd.GetTx().Inputs[0].Address) {
					return errors.Errorf("%s Input hash index: %s. Tx hash: %s, double hash: %s", ErrInputExists, key, firstTxHash, hash)
				}

				// If given tx has the same num and bigger fee add it to the pool
				// and remove first tx from the pool.
				if td.Num() == ftd.Num() && td.Fee() > ftd.Fee() {
					err := tp.deleteTransactionByHash(firstTxHash)
					if err != nil {
						return err
					}

					log.Infof("Swap tx %s with tx %s in the pool.", firstTxHash, hash)

					return nil
				}
			}

			//td.AddAlias(firstTxHash) // link current Transaction to the first tx
			//tp.villainousPool[firstTxHash] = td

			return errors.Errorf("%s Input hash index: %s. Tx hash: %s, double hash: %s", ErrInputExists, key, firstTxHash, hash)
		}
	}

	return nil
}

func (tp *TxPool) GetTxQueue() []*types.TransactionData {
	tp.lock.Lock()
	defer tp.lock.Unlock()

	// sort data before return
	tp.pricedPool.Sort()

	return tp.pricedPool
}

// ReserveTransactions set status of given tx batch as reserved for block
func (tp *TxPool) ReserveTransactions(arr []*prototype.Transaction) error {
	tp.lock.Lock()
	defer tp.lock.Unlock()

	for _, tx := range arr {
		if tx.Type == common.RewardTxType {
			continue
		}

		hash := common.Encode(tx.Hash)

		td, exists := tp.pool[hash]
		_, reservExists := tp.reservedPool[hash]

		if !exists {
			log.Errorf("TxPool.ReserveTransactions: Not found transaction %s.", hash)
			return ErrTxNotFound
		}

		// if tx is already reserved block cann't use it
		if reservExists {
			log.Errorf("TxPool.ReserveTransactions: Tx %s already reserved.", hash)
			return ErrAlreadyReserved
		}

		// switch tx to the reserved status
		tp.reservedPool[hash] = td // copy to reserved pool
		delete(tp.pool, hash)

		// delete from priced pool
		err := tp.DeleteFromPricedPool(tx)
		if err != nil {
			log.Errorf("TxPool.ReserveTransactions: Not found in priced pool tx %s.", hash)
			return ErrTxNotFoundInPricedPool
		}
	}

	return nil
}

// FlushReserved reset reserved for block transactions
func (tp *TxPool) FlushReserved(cleanInputs bool) {
	tp.lock.Lock()
	if cleanInputs {
		for _, txd := range tp.reservedPool {
			tp.unlockInputs(txd.GetTx())
		}
	}

	tp.reservedPool = map[string]*types.TransactionData{}
	tp.lock.Unlock()
}

// unlockInputs delete transaction inputs
func (tp *TxPool) unlockInputs(tx *prototype.Transaction) {
	var key string
	hash := common.Encode(tx.Hash)
	for _, in := range tx.Inputs {
		key = genKeyFromInput(in)
		_, exists := tp.lockedInputs[key]

		if !exists {
			log.Warnf("Trying to delete unexist input key %s.", key)
			continue
		}

		delete(tp.lockedInputs, key)

		log.Debugf("Unlock input %s for tx %s", key, hash)
	}
}

// RollbackReserved reset reserved for block transactions
func (tp *TxPool) RollbackReserved() {
	tp.lock.Lock()

	for hash, txd := range tp.reservedPool {
		_, exists := tp.pool[hash]

		if exists {
			log.Warnf("Can't return tx %s to the pool because it is already exists there.", hash)

			continue
		}

		tp.pool[hash] = txd
		tp.pricedPool = append(tp.pricedPool, txd)
	}

	tp.lock.Unlock()

	// reset pool
	tp.FlushReserved(false)
}

// DeleteFromPricedPool deletes given tx from pricedPool
func (tp *TxPool) DeleteFromPricedPool(tx *prototype.Transaction) error {
	// get given tx index
	index, err := tp.pricedPool.FindByTx(tx)
	if err != nil {
		return err
	}

	// delete found element
	tp.pricedPool = DeleteFromPricedPool(tp.pricedPool, index)

	return nil
}

// deleteTransactionByHash
func (tp *TxPool) deleteTransactionByHash(hash string) error {
	td, exists := tp.pool[hash]

	if !exists {
		return errors.Errorf("Not found tx with hash %s in pool", hash)
	}

	tx := td.GetTx()

	delete(tp.pool, hash)
	tp.unlockInputs(tx)

	err := tp.DeleteFromPricedPool(tx)
	if err != nil {
		log.Errorf("Error deleting stake transaction %s", hash)
		return err
	}

	return nil
}

// DeleteTransaction delete transaction from the pool
func (tp *TxPool) DeleteTransaction(tx *prototype.Transaction) error {
	hash := common.BytesToHash(tx.Hash).Hex()

	tp.lock.Lock()
	defer tp.lock.Unlock()

	return tp.deleteTransactionByHash(hash)
}

// checkTxOut func for verifying tx was deleted from pool correctly
func (tp *TxPool) checkTxOut(tx *prototype.Transaction) bool {
	tp.lock.Lock()
	defer tp.lock.Unlock()

	hash := common.Encode(tx.Hash)
	for _, val := range tp.lockedInputs {
		fhash := strings.Split(val, "_")[0]

		if fhash == hash {
			return true
		}
	}

	_, exists := tp.pool[hash]
	if exists {
		return true
	}

	_, exists = tp.reservedPool[hash]
	if exists {
		return true
	}

	_, exists = tp.villainousPool[hash]
	if exists {
		log.Warnf("Tx %s exists in double spend pool.", hash)
	}

	index, err := tp.pricedPool.FindByTx(tx)
	if err != nil {
		log.Errorf("Error searching tx %s", err)
		return true
	}

	if index > -1 {
		return true
	}

	return false
}

// StopWriting stop all writing operations
func (tp *TxPool) StopWriting() {
	tp.finish()
}

// GetFee return minimal fee needed for adding to the block.
func (tp *TxPool) GetFee() uint64 {
	queue := tp.GetTxQueue()
	size := len(queue)

	if size == 0 {
		return tp.cfg.MinimalFee
	}

	var txSize, totalSize, i int
	for i = 0; i < size; i++ {
		txSize = queue[i].Size()
		totalSize += txSize

		if totalSize <= tp.cfg.BlockSize {
			// we fill block successfully
			if totalSize == tp.cfg.BlockSize {
				break
			}
		} else {
			// tx is too big try for look up another one
			totalSize -= size
		}
	}

	if i == size {
		i--
	}

	minFee := queue[i].Fee()
	maxFee := queue[0].Fee()

	fee := (maxFee + minFee) / 2

	return fee
}

func (tp *TxPool) GetPendingTransactions() ([]*prototype.Transaction, error) {
	queue := tp.GetTxQueue()

	batch := make([]*prototype.Transaction, len(queue))
	for i, td := range queue {
		batch[i] = td.GetTx()
	}

	return batch, nil
}

func genKeyFromInput(in *prototype.TxInput) string {
	return common.Encode(in.Hash) + "_" + strconv.Itoa(int(in.Index))
}

type pricedTxPool []*types.TransactionData

func (ptp pricedTxPool) Len() int {
	return len(ptp)
}

func (ptp pricedTxPool) Less(i, j int) bool {
	// if fee price is equal than compare timestamp
	// bigger timestamp is worse
	if ptp[i].Fee() == ptp[j].Fee() {
		return ptp[i].Timestamp() < ptp[j].Timestamp()
	}

	return ptp[i].Fee() > ptp[j].Fee()
}

func (ptp pricedTxPool) Swap(i, j int) {
	ptp[i], ptp[j] = ptp[j], ptp[i]
}

func (ptp pricedTxPool) Sort() {
	sort.Slice(ptp, ptp.Less)
}

// FindByTx find tx index in priced pool
func (ptp pricedTxPool) FindByTx(tx *prototype.Transaction) (int, error) {
	for i, td := range ptp {
		if bytes.Equal(tx.Hash, td.GetTx().Hash) {
			return i, nil
		}
	}

	return -1, ErrTxNotFoundInPricedPool
}

func DeleteFromPricedPool(ptp pricedTxPool, index int) pricedTxPool {
	if index >= ptp.Len() || index < 0 {
		return ptp
	}

	last := len(ptp) - 1
	ptp[index] = ptp[last]
	return ptp[:last]
}
