package txpool

import (
	"bytes"
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/sirupsen/logrus"
	"sort"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrTxExists               = errors.New("tx already exists in pool")
	ErrInputExists            = errors.New("tx input is locked for spend")
	ErrTxNotFound             = errors.New("tx was not found in the pool")
	ErrTxNotFoundInPricedPool = errors.New("tx was not found in priced pool")
	ErrPoolClosed             = errors.New("tx pool is closed for writing")
)

const (
	TxWaitStatus     = iota // tx is valid and wait it's turn to be add to the block
	TxReservedStatus        // tx added to the block
)

var log = logrus.WithField("prefix", "TxPool")

func NewTxPool(v consensus.TxValidator) *TxPool {
	ctx, finish := context.WithCancel(context.Background())

	tp := TxPool{
		validator:      v,
		pool:           map[string]*TransactionData{},
		lockedInputs:   map[string]string{},
		pricedPool:     make(pricedTxPool, 0),
		reservedPool:   map[string]*TransactionData{},
		villainousPool: map[string]*TransactionData{},

		// ctx
		ctx:    ctx,
		finish: finish,

		// channel
		dataC: make(chan *prototype.Transaction),
	}

	// start reading tx loop
	go tp.ReadingLoop()

	return &tp
}

type TxPool struct {
	lock sync.RWMutex

	validator consensus.TxValidator

	// to mark that inputs has been already spent
	// and avoid double spend
	lockedInputs map[string]string

	// valid tx pool map[tx hash] -> tx
	pool map[string]*TransactionData

	// priced list
	pricedPool pricedTxPool

	// reserved tx list for future block
	reservedPool map[string]*TransactionData

	// double spend tx
	villainousPool map[string]*TransactionData

	dataC chan *prototype.Transaction

	ctx    context.Context
	finish context.CancelFunc
}

// SendTx add tx to the pool
func (tp *TxPool) SendTx(tx *prototype.Transaction) error {
	select {
	case <-tp.ctx.Done():
		// got interrupt stop
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
			tp.StopWriting()
		}
	}
}

// RegisterTx validate tx and add it to the pool if it is correct
func (tp *TxPool) RegisterTx(tx *prototype.Transaction) error {
	td := NewTxData(tx)

	err := tp.validateTx(td)
	if err != nil {
		log.Errorf("RegisterTx: %s", err)
		return err
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
	}
	tp.lock.Unlock()

	return nil
}

// validateTx validates tx by validator, finds double spends and checks tx exists in the pool
func (tp *TxPool) validateTx(td *TransactionData) error {
	txHash := common.Encode(td.GetTx().Hash)

	// validate balance, signatures and hash check
	err := tp.validator.ValidateTransactionStruct(td.GetTx())
	if err != nil {
		return err
	}

	// check tx is in pool already
	tp.lock.RLock()
	_, exists := tp.pool[txHash]
	_, reserved := tp.reservedPool[txHash]
	tp.lock.RUnlock()

	if exists || reserved {
		return ErrTxExists
	}

	err = tp.validator.ValidateTransaction(td.GetTx())
	if err != nil {
		return err
	}

	tp.lock.Lock()
	defer tp.lock.Unlock()

	hash := common.Encode(td.GetTx().Hash)

	// lock inputs
	for _, in := range td.GetTx().Inputs {
		key := genKeyFromInput(in)
		firstTxHash, exists := tp.lockedInputs[key]

		if exists {
			td.SetBro(firstTxHash) // link current Transaction to the first tx
			tp.villainousPool[firstTxHash] = td

			return errors.Errorf("%s Input hash index: %s. Tx hash: %s, double hash: %s", ErrInputExists, key, firstTxHash, hash)
		}
	}

	return nil
}

// GetPricedQueue returns all transactions sorted by fee value
func (tp *TxPool) GetPricedQueue() []*TransactionData {
	tp.lock.RLock()
	defer tp.lock.RUnlock()

	// sort data before return
	tp.pricedPool.Sort()

	return tp.pricedPool
}

// ReserveTransactions set status of given tx batch as reserved for block
func (tp *TxPool) ReserveTransactions(arr []*prototype.Transaction) error {
	for _, tx := range arr {
		hash := common.Encode(tx.Hash)

		tp.lock.RLock()
		td, exists := tp.pool[hash]
		_, reservExists := tp.reservedPool[hash]
		tp.lock.RUnlock()

		if !exists {
			return ErrTxNotFound
		}

		// if tx is already reserved block cann't use it
		if reservExists {
			return errors.Errorf("Tx %s already reserved.", hash)
		}

		// change tx status
		td.SetStatus(TxReservedStatus)

		// switch tx to the reserved status
		tp.lock.Lock()
		tp.reservedPool[hash] = td // copy to reserved pool
		delete(tp.pool, hash)
		tp.lock.Unlock()

		// delete from priced pool
		err := tp.DeleteFromPricedPool(tx)
		if err != nil {
			log.Warn(err)
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

	tp.reservedPool = map[string]*TransactionData{}
	tp.lock.Unlock()
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

		txd.SetStatus(TxWaitStatus)
		tp.pool[hash] = txd
		tp.pricedPool = append(tp.pricedPool, txd)
	}

	tp.lock.Unlock()

	// reset pool
	tp.FlushReserved(false)
}

// DeleteFromPricedPool deletes given tx from pricedPool
func (tp *TxPool) DeleteFromPricedPool(tx *prototype.Transaction) error {
	tp.lock.Lock()
	defer tp.lock.Unlock()

	// get given tx index
	index, err := tp.pricedPool.FindByTx(tx)
	if err != nil {
		return err
	}

	// delete found element
	tp.pricedPool.Delete(index)

	return nil
}

// DeleteTransaction delete transaction from the pool
func (tp *TxPool) DeleteTransaction(tx *prototype.Transaction) error {
	hash := common.BytesToHash(tx.Hash).Hex()

	tp.lock.Lock()
	defer tp.lock.Unlock()

	return tp.deleteTransactionByHash(hash)
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

// unlockInputs delete transaction inputs
func (tp *TxPool) unlockInputs(tx *prototype.Transaction) {
	for _, in := range tx.Inputs {
		key := genKeyFromInput(in)
		_, exists := tp.lockedInputs[key]

		if !exists {
			log.Warnf("Trying to delete unexist input key %s.", key)
			continue
		}

		delete(tp.lockedInputs, key)
	}
}

// checkTxOut func for verifying tx was deleted from pool correctly
func (tp *TxPool) checkTxOut(tx *prototype.Transaction) bool {
	tp.lock.RLock()
	defer tp.lock.RUnlock()

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
	//close(tp.dataC)
}

func genKeyFromInput(in *prototype.TxInput) string {
	return common.Encode(in.Hash) + "_" + strconv.Itoa(int(in.Index))
}

type pricedTxPool []*TransactionData

func (ptp pricedTxPool) Len() int {
	return len(ptp)
}

func (ptp pricedTxPool) Less(i, j int) bool {
	// if fee price is equal than compare timestamp
	// bigger timestamp is worse
	if ptp[i].tx.Fee == ptp[j].tx.Fee {
		return ptp[i].tx.Timestamp < ptp[j].tx.Timestamp
	}

	return ptp[i].tx.Fee > ptp[j].tx.Fee
}

func (ptp pricedTxPool) Swap(i, j int) {
	ptp[i], ptp[j] = ptp[j], ptp[i]
}

func (ptp pricedTxPool) Sort() {
	sort.Slice(ptp, ptp.Less)
}

func (ptp pricedTxPool) FindByTx(tx *prototype.Transaction) (int, error) {
	left := 0
	right := len(ptp) - 1

	less := func(tx1, tx2 *prototype.Transaction) bool {
		if tx1.Fee == tx2.Fee {
			return tx1.Timestamp < tx2.Timestamp
		}

		return tx1.Fee > tx2.Fee
	}

	notFound := true

	var mid int
	var el *prototype.Transaction

	for left <= right {
		mid = (left + right) / 2
		el = ptp[mid].GetTx()

		if bytes.Equal(el.Hash, tx.Hash) {
			notFound = false
			break
		}

		if less(tx, el) {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	if notFound {
		return -1, ErrTxNotFoundInPricedPool
	}

	return mid, nil
}

// Delete removes element with given index from slice
func (ptp *pricedTxPool) Delete(i int) {
	if i >= len(*ptp) || i < 0 {
		return
	}

	*ptp = append((*ptp)[:i], (*ptp)[i+1:]...)
}

type TransactionData struct {
	tx      *prototype.Transaction
	size    int // counted tx size for sort
	status  int
	brother string
	checked bool
}

func (td *TransactionData) GetTx() *prototype.Transaction {
	return td.tx
}

func (td *TransactionData) Size() int {
	return td.size
}

func (td *TransactionData) SetStatus(code int) {
	td.status = code
}

func (td *TransactionData) GetStatus() int {
	return td.status
}

func (td *TransactionData) SetBro(hash string) {
	td.brother = hash
}

func (td *TransactionData) SetChecked() {
	td.checked = true
}

func (td *TransactionData) UnsetChecked() {
	td.checked = false
}

func (td *TransactionData) IsChecked() bool {
	return td.checked
}

func NewTxData(tx *prototype.Transaction) *TransactionData {
	td := TransactionData{
		tx:   tx,
		size: tx.SizeSSZ(),
	}

	return &td
}
