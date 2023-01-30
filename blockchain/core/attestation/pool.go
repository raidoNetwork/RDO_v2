package attestation

import (
	"bytes"
	"sort"
	"sync"

	"github.com/raidoNetwork/RDO_v2/shared/params"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/utils/async"
	utypes "github.com/raidoNetwork/RDO_v2/utils/types"
	"github.com/sirupsen/logrus"
)

const (
	maxTxCount = 1000
)

var log = logrus.WithField("prefix", "attestation")

type PoolSettings struct {
	Validator  consensus.TxValidator
	MinimalFee uint64
}

func NewPool(cfg *PoolSettings) *Pool {
	return &Pool{
		txSenderMap: map[string]*types.Transaction{},
		txHashMap:   map[string]*types.Transaction{},
		stakeTxMap:  map[string]*types.Transaction{},
		pending:     make(Transactions, 0),
		cfg:         cfg,
	}
}

type Pool struct {
	txSenderMap map[string]*types.Transaction
	txHashMap   map[string]*types.Transaction
	stakeTxMap  map[string]*types.Transaction

	pending Transactions
	cfg     *PoolSettings

	mu        sync.Mutex
	queueLock sync.Mutex

	swapLock async.Mutex
}

func (p *Pool) Insert(tx *types.Transaction) error {
	p.mu.Lock()

	hash := tx.Hash().Hex()
	if _, exists := p.txHashMap[hash]; exists {
		p.mu.Unlock()
		return errors.New("Already exists")
	}

	sender := tx.From().Hex()
	if sender == "" {
		return errors.New("Wrong sender")
	}

	if oldTx, exists := p.txSenderMap[sender]; exists {
		p.mu.Unlock()
		return p.processDoubleSpend(oldTx, tx)
	}

	p.mu.Unlock()

	err := p.validateTx(tx)
	if err != nil {
		return err
	}

	p.finalizeInsert(tx)
	log.Debugf("Insert tx %s", tx.Hash().Hex())

	if len(p.txHashMap)+1 == maxTxCount {
		p.cleanWorst()
	}

	return nil
}

func (p *Pool) processDoubleSpend(oldTx, newTx *types.Transaction) error {
	if oldTx.Type() == common.CollapseTxType {
		return errors.New("Outputs cleaning. Try later")
	}

	if oldTx.Num() == newTx.Num() {
		canSwap := txLess(oldTx, newTx)
		if !canSwap {
			return errors.New("Too cheap tx")
		}

		err := p.validateTx(newTx)
		if err != nil {
			return err
		}

		return p.swap(oldTx, newTx)
	}
	return nil
}

func (p *Pool) swap(oldTx, newTx *types.Transaction) error {
	p.swapLock.WaitLock()
	if oldTx.IsForged() {
		return nil
	}

	p.queueLock.Lock()
	err := p.pending.SwapByHash(oldTx, newTx)
	if err != nil {
		p.queueLock.Unlock()
		return err
	}
	p.queueLock.Unlock()

	p.mu.Lock()

	// If a stake tx, update stakeTxMap
	if newTx.Type() == common.StakeTxType {
		p.stakeTxMap[newTx.From().Hex()] = newTx
	}

	if oldTx.Type() == common.StakeTxType {
		delete(p.stakeTxMap, oldTx.From().Hex())
	}

	delete(p.txHashMap, oldTx.Hash().Hex())
	p.txHashMap[newTx.Hash().Hex()] = newTx
	p.txSenderMap[newTx.From().Hex()] = newTx
	p.mu.Unlock()

	log.Debugf("Swap %s with %s", oldTx.Hash().Hex(), newTx.Hash().Hex())

	return nil
}

func (p *Pool) finalizeInsert(tx *types.Transaction) {
	p.mu.Lock()
	if tx.Type() == uint32(common.StakeTxType) {
		p.stakeTxMap[tx.From().Hex()] = tx
	}
	p.txSenderMap[tx.From().Hex()] = tx
	p.txHashMap[tx.Hash().Hex()] = tx
	p.mu.Unlock()

	p.queueLock.Lock()
	p.pending = append(p.pending, tx)
	p.queueLock.Unlock()
}

func (p *Pool) cleanWorst() {
	queue := p.GetQueue()
	lastIndex := len(queue) - 1
	worst := queue[lastIndex]

	p.mu.Lock()
	delete(p.txHashMap, worst.Hash().Hex())
	delete(p.txSenderMap, worst.From().Hex())
	p.mu.Unlock()

	p.queueLock.Lock()
	p.pending = p.pending[:lastIndex]
	p.queueLock.Unlock()

	worst.Drop()
	log.Debugf("Delete worst %s", worst.Hash().Hex())
}

func (p *Pool) validateTx(tx *types.Transaction) error {
	err := p.cfg.Validator.ValidateTransactionStruct(tx)
	if err != nil {
		return err
	}

	// todo check locked inputs here
	if tx.Type() == uint32(common.StakeTxType) {
		if err := p.cfg.Validator.ValidateTransaction(tx); err != nil {
			return err
		}

		return p.CheckMaxStakers(tx)
	} else {
		return p.cfg.Validator.ValidateTransaction(tx)
	}
}

func (p *Pool) CheckMaxStakers(tx *types.Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cfg.Validator.CheckMaxStakers(tx, len(p.stakeTxMap))
}

func (p *Pool) InsertCollapseTx(txs []*types.Transaction) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, tx := range txs {
		if tx.Type() != common.CollapseTxType {
			return errors.New("Wrong tx type given")
		}

		senders := map[string]struct{}{}
		for _, in := range tx.Inputs() {
			from := in.Address().Hex()
			if _, exists := senders[from]; exists {
				continue
			}

			if _, exists := p.txSenderMap[from]; exists {
				return errors.New("CollapseTx can trigger double spend")
			}

			senders[from] = struct{}{}
		}

		for from := range senders {
			p.txSenderMap[from] = tx
		}

		p.txHashMap[tx.Hash().Hex()] = tx
	}

	return nil
}

func (p *Pool) GetQueue() []*types.Transaction {
	p.queueLock.Lock()
	defer p.queueLock.Unlock()

	// sort data before return
	p.pending.Sort()

	return p.pending
}

func (p *Pool) GetFeePrice() uint64 {
	queue := p.GetQueue()
	size := len(queue)
	if size == 0 {
		return params.RaidoConfig().MinimalFee
	} else if size == 1 {
		return queue[0].FeePrice()
	} else {
		return (queue[0].FeePrice() + queue[size-1].FeePrice()) / 2
	}
}

func (p *Pool) Finalize(txarr []*types.Transaction) {
	p.mu.Lock()
	// Gather ValidatorsUnstakeTxs
	validatorsUnstakes := make([]*types.Transaction, 0)

	for _, tx := range txarr {
		if tx.Type() == common.ValidatorsUnstakeTxType {
			validatorsUnstakes = append(validatorsUnstakes, tx)
		}

		if utypes.IsSystemTx(tx) && tx.Type() != common.CollapseTxType {
			continue
		}

		rtx, index, err := p.findPoolTransaction(tx)
		if err != nil {
			continue
		}

		p.queueLock.Lock()
		p.pending = p.pending.SwapAndRemove(index)
		p.queueLock.Unlock()

		p.cleanTransactionMap(rtx)
	}

	// Clean up SystemUnstakeTxs
	for _, tx := range validatorsUnstakes {
		p.cleanUpSystemUnstake(tx)
	}

	p.mu.Unlock()
}

func (p *Pool) findPoolTransaction(tx *types.Transaction) (*types.Transaction, int, error) {
	_, exists := p.txHashMap[tx.Hash().Hex()]
	senderTx, senderExists := p.txSenderMap[tx.From().Hex()]

	log.Debugf("Looking for Tx %s HashMap %v SenderMap %v Sender %s", tx.Hash().Hex(), exists, senderExists, tx.From().Hex())

	if !exists && !senderExists {
		return nil, -1, errors.New("Undefined sender and transaction")
	}

	// now tx can be one of several cases:
	//  1. tx exists in pool
	//  2. tx is double of previous sender tx
	// 	3. tx is the first swapped transaction so it does exist in the pool
	// 	AND it is not marked as double
	var poolTx *types.Transaction
	if exists {
		poolTx = tx
	} else {
		poolTx = senderTx
	}

	index := p.pending.GetIndex(poolTx)
	if index == -1 && poolTx.Type() != common.CollapseTxType {
		return nil, -1, errors.New("Undefined pending transaction")
	}

	return poolTx, index, nil
}

func (p *Pool) cleanTransactionMap(tx *types.Transaction) {
	delete(p.txHashMap, tx.Hash().Hex())

	if tx.Type() != common.CollapseTxType {
		delete(p.txSenderMap, tx.From().Hex())
	} else {
		for _, sender := range tx.AllSenders() {
			delete(p.txSenderMap, sender.Hex())
		}
	}

	if tx.Type() == common.StakeTxType {
		delete(p.stakeTxMap, tx.From().Hex())
	}
}

func (p *Pool) findProblematicStakeTx(address, node string) (*types.Transaction, int, error) {
	tx, exists := p.txSenderMap[address]
	if !exists {
		return nil, -1, errors.Errorf("No problematic transaction found in tx pool")
	}

	// Checking if the transaction is a stake transaction on the specified node
	for _, out := range tx.Outputs() {
		if out.Node().Hex() == node {
			index := p.pending.GetIndex(tx)
			return tx, index, nil
		}
	}

	// Checking if the transaction is an unstake transaction from the specified node
	for _, in := range tx.Inputs() {
		if in.Node().Hex() == node {
			index := p.pending.GetIndex(tx)
			return tx, index, nil
		}
	}

	return nil, -1, errors.Errorf("No problematic transaction found in tx pool")
}

func (p *Pool) cleanUpSystemUnstake(tx *types.Transaction) {
	// For address, remove the corresponding stake and unstake transactions
	// from the transaction pool
	for _, in := range tx.Inputs() {
		address := in.Address().Hex()
		node := in.Node().Hex()
		rtx, index, err := p.findProblematicStakeTx(address, node)
		if err != nil {
			log.Debugf("Error in cleanUpSystemUnstake: %s\n", err)
			continue
		}
		p.queueLock.Lock()
		p.pending = p.pending.SwapAndRemove(index)
		p.queueLock.Unlock()

		p.cleanTransactionMap(rtx)
	}
}

func (p *Pool) DeleteTransaction(tx *types.Transaction) error {
	found := false
	for i, ptx := range p.pending {
		if bytes.Equal(ptx.Hash(), tx.Hash()) {
			p.pending = p.pending.SwapAndRemove(i)
			found = true
			break
		}
	}

	if !found {
		return errors.Errorf("Not found tx %s", tx.Hash().Hex())
	}

	p.mu.Lock()
	p.cleanTransactionMap(tx)
	p.mu.Unlock()

	log.Debugf("Delete transaction %s", tx.Hash().Hex())

	return nil
}

func (p *Pool) LockPool() {
	p.swapLock.Lock()
}

func (p *Pool) UnlockPool() {
	p.swapLock.Unlock()
}

func (p *Pool) ClearForged(block *prototype.Block) {
	p.mu.Lock()
	p.queueLock.Lock()
	for _, ptx := range block.Transactions {
		hash := common.Encode(ptx.Hash)
		tx, exists := p.txHashMap[hash]
		if !exists {
			continue
		}

		if tx.IsForged() {
			tx.DiscardForge()
		}
	}
	p.queueLock.Unlock()
	p.mu.Unlock()
}

type Transactions []*types.Transaction

func (txs Transactions) Less(i, j int) bool {
	return txLess(txs[i], txs[j])
}

func (txs Transactions) Swap(i, j int) {
	txs[i], txs[j] = txs[j], txs[i]
}

func (txs Transactions) Sort() {
	sort.Slice(txs, txs.Less)
}

func (txs Transactions) SwapByHash(old, new *types.Transaction) error {
	for i, tx := range txs {
		if bytes.Equal(old.Hash(), tx.Hash()) {
			txs[i] = new
			return nil
		}
	}

	return errors.New("Not found tx")
}

func (txs Transactions) SwapAndRemove(i int) Transactions {
	if i < 0 || i >= len(txs) {
		return txs
	}

	last := len(txs) - 1
	txs.Swap(i, last)
	return txs[:last]
}

func (txs Transactions) GetIndex(tx *types.Transaction) int {
	for i, stx := range txs {
		if bytes.Equal(stx.Hash(), tx.Hash()) {
			return i
		}
	}

	return -1
}

func txLess(a, b *types.Transaction) bool {
	if a.FeePrice() == b.FeePrice() {
		return a.Timestamp() < b.Timestamp()
	}

	return a.FeePrice() > b.FeePrice()
}
