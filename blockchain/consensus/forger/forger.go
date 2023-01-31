package forger

import (
	"time"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus/backend/pos"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "Forger")

var (
	ErrZeroFeeAmount = errors.New("Block has no transactions with fee.")
)

const (
	txBatchLimit      = 1000
	InputsPerTxLimit  = 2000
	OutputsPerTxLimit = 2000
	collapseTxLimit   = 30
)

type Config struct {
	EnableMetrics bool
	BlockSize     int
	Proposer      *keystore.ValidatorAccount
	Engine        *pos.Backend
}

func New(bc consensus.BlockFinalizer, att consensus.AttestationPool, cfg *Config) *Forger {
	m := Forger{
		bf:       bc,
		att:      att,
		cfg:      cfg,
		skipAddr: map[string]struct{}{},
	}

	return &m
}

type Forger struct {
	bf       consensus.BlockFinalizer
	att      consensus.AttestationPool
	cfg      *Config
	skipAddr map[string]struct{}
}

// ForgeBlock create block from tx pool data
func (m *Forger) ForgeBlock() (*prototype.Block, error) {
	start := time.Now()
	bn := m.bf.GetBlockCount()
	totalSize := 0 // current size of block in bytes

	// lock pool
	m.att.TxPool().LockPool()
	defer m.att.TxPool().UnlockPool()

	txQueueOrigin := m.att.TxPool().GetQueue()
	txQueue := make([]*types.Transaction, len(txQueueOrigin))
	copy(txQueue, txQueueOrigin)

	txQueueLen := len(txQueue)
	txBatch := make([]*prototype.Transaction, 0, txQueueLen)
	collapseBatch := make([]*prototype.Transaction, 0, txQueueLen)

	// map of full validator unstakes
	fullUnstakes := make(map[string]struct{})
	systemUnstakes := make([]*prototype.Transaction, 0)

	pendingTxCounter.Set(float64(txQueueLen)) // todo remove metrics to the core service

	// create reward transactions for current block
	// txBatch, collapseBatch, totalSize, err := m.addRewardTxToBatch(m.cfg.Proposer.Addr().Hex(), txBatch, collapseBatch, totalSize, bn)
	txBatch, collapseBatch, totalSize, err := m.addRewardsTxToBatch(m.cfg.Proposer.Addr().Hex(), txBatch, collapseBatch, totalSize, bn)
	if err != nil {
		return nil, err
	}

	// limit tx count in block according to marshaller settings
	feeTxCount := 1
	var txCountPerBlock int
	if txQueueLen < txBatchLimit {
		txCountPerBlock = txQueueLen + len(txBatch) + len(collapseBatch) + feeTxCount
	} else {
		txCountPerBlock = txBatchLimit - len(txBatch) - len(collapseBatch) - feeTxCount
	}

	var size int
	txType := "StandardTx"
MAINLOOP:
	for _, tx := range txQueue {
		if len(txBatch)+len(collapseBatch)+len(systemUnstakes) == txCountPerBlock {
			break
		}

		if tx.IsDropped() {
			log.Debugf("Skip dropped tx %s", tx.Hash().Hex())
			continue
		}

		size = tx.Size()
		totalSize += size

		// tx is too big try for look up another one
		if totalSize > m.cfg.BlockSize {
			totalSize -= size
			break
		}

		hash := tx.Hash().Hex()

		// check if empty validator slots exists and skip stake tx if not exist
		if tx.Type() == common.StakeTxType {
			var amount uint64
			hasStake := false
			for _, out := range tx.Outputs() {
				if len(out.Node()) == common.AddressLength {
					hasStake = true
				}

				if out.Node().Hex() == common.BlackHoleAddress {
					if m.cfg.Engine.IsValidator(tx.From()) {
						amount += out.Amount()
					} else {
						err = errors.New("Unauthorized slot reservation")
						break
					}
				}

				// a validator cannot stake on any validator (including itself)
				if m.cfg.Engine.IsValidator(out.Node()) && m.cfg.Engine.IsValidator(tx.From()) {
					err = errors.New("A validator can only stake on the BlackHoleAddress")
					break
				}
			}

			if amount > 0 && err == nil {
				err = m.att.StakePool().ReserveSlots(amount)
			}

			if err != nil || !hasStake {
				totalSize -= size // return size of current tx

				// Delete stake tx from pool
				if err := m.att.TxPool().DeleteTransaction(tx); err != nil {
					return nil, errors.Wrap(err, "Error creating block")
				}

				log.Warnf("Skip stake tx %s: %s", hash, err)
				continue
			}

			txType = "StakeTx"
		}

		// If full validator unstake, create a SystemUnstakeTx
		if tx.Type() == common.UnstakeTxType && m.cfg.Engine.IsValidator(tx.From()) {
			fullUnstake := true
			node := tx.From().Hex()
			for _, out := range tx.Outputs() {
				if out.Node().Hex() == node {
					fullUnstake = false
					break
				}
			}
			if fullUnstake {
				fullUnstakes[node] = struct{}{}
				txs, err := m.generateSystemUnstakes(node, m.bf.GetBlockCount())
				if err != nil {
					return nil, err
				}

				txsSize := countTXsSize(txs)

				// If exceeds the limit, skip the validator unstake transaction
				// and delete from fullUnstakes
				if txsSize+totalSize >= m.cfg.BlockSize {
					delete(fullUnstakes, node)
					continue MAINLOOP
				}

				if len(txBatch)+len(collapseBatch)+len(systemUnstakes) >= txCountPerBlock {
					continue MAINLOOP
				}

				totalSize += txsSize

				systemUnstakes = append(systemUnstakes, txs...)
			}

		}

		txBatch = append(txBatch, tx.GetTx())
		tx.Forge()
		log.Debugf("Add %s %s to the block %d", txType, hash, bn)
		txType = "StandardTx"

		collapseTx, err := m.createCollapseTx(tx, m.bf.GetBlockCount(), totalSize)
		if err != nil {
			log.Errorf("Can't collapse tx %s outputs", hash)
			return nil, errors.Wrap(err, "Error collapsing tx outputs")
		}

		// no need to create collapse tx for given tx addresses
		leftCollapseBatch := make([]*prototype.Transaction, 0, len(collapseTx))
		if collapseTx != nil {
			for _, colTx := range collapseTx {
				if totalSize+colTx.Size() >= m.cfg.BlockSize {
					break
				}
				leftCollapseBatch = append(leftCollapseBatch, colTx.GetTx())
				totalSize += colTx.Size()
				log.Debugf("CollapseTx %s created", colTx.Hash().Hex())

			}
			err := m.att.TxPool().InsertCollapseTx(collapseTx)
			if err != nil {
				log.Error(err)
			} else {
				// Check for size when appending collapseTxs
				collapseBatch = append(collapseBatch, leftCollapseBatch...)
			}

		}
	}

	// add collapsed transaction to the end of the batch
	txBatch = append(txBatch, collapseBatch...)

	// Cleaning up according to fullUnstakes map.
	// We want to remove any stake or unstake transaction on particular nodes
	if len(fullUnstakes) > 0 {
		cleanBatch := make([]*prototype.Transaction, 0, len(txBatch))
		for _, tx := range txBatch {
			// If the node parameter in outputs is in fullUnstakes map,
			// remove the transaction from the batch
			bad := isBadTransaction(tx, fullUnstakes)
			if !bad {
				cleanBatch = append(cleanBatch, tx)
			}
		}
		txBatch = cleanBatch
	}

	// Add system unstake transactions to the batch
	txBatch = append(systemUnstakes, txBatch...)

	// generate fee tx for block
	if len(txBatch) > 0 {
		txFee, err := m.createFeeTx(txBatch)

		if err != nil {
			if !errors.Is(err, ErrZeroFeeAmount) {
				return nil, err
			}

			log.Debug(err)
		} else {
			txBatch = append(txBatch, txFee)
			log.Debugf("Add FeeTx %s to the block", common.Encode(txFee.Hash))
		}
	}

	// clear collapse list
	m.skipAddr = map[string]struct{}{}

	// get block instance
	block := types.NewBlock(m.bf.GetBlockCount(), slot.Ticker().Slot(), m.bf.ParentHash(), txBatch, m.cfg.Proposer)

	end := time.Since(start)
	log.Warnf("Generate block with transactions count: %d. TxPool transactions count: %d. Size: %d kB. Time: %s", len(txBatch), txQueueLen, totalSize/1024, common.StatFmt(end))
	updateForgeMetrics(end)

	return block, nil
}

func (m *Forger) addRewardsTxToBatch(proposer string, txBatch []*prototype.Transaction, collapseBatch []*prototype.Transaction, totalSize int, bn uint64) ([]*prototype.Transaction, []*prototype.Transaction, int, error) {
	rewardTxs, err := m.createRewardTxs(m.bf.GetBlockCount(), proposer)
	if err != nil {
		if errors.Is(err, consensus.ErrNoStakers) {
			log.Debug(err)
			return txBatch, collapseBatch, totalSize, nil
		} else {
			return nil, nil, 0, err
		}
	}

	for _, rewardTx := range rewardTxs {
		txBatch = append(txBatch, rewardTx)
		totalSize += rewardTx.SizeSSZ()

		log.Debugf("Add RewardTx %s to the block", common.Encode(rewardTx.Hash))

		collapseTx, err := m.createCollapseTx(types.NewTransaction(rewardTx), m.bf.GetBlockCount(), totalSize)
		if err != nil {
			log.Errorf("Can't collapse reward tx %s outputs", common.Encode(rewardTx.Hash))
			return nil, nil, 0, errors.Wrap(err, "Error collapsing tx outputs")
		}

		if collapseTx != nil {
			err := m.att.TxPool().InsertCollapseTx(collapseTx)
			if err != nil {
				return txBatch, collapseBatch, totalSize, nil
			}

			for _, coltx := range collapseTx {
				collapseBatch = append(collapseBatch, coltx.GetTx())
				totalSize += coltx.Size()
				log.Debugf("Add CollapseTx %s to the block %d", coltx.Hash().Hex(), bn)
			}

		}
	}

	return txBatch, collapseBatch, totalSize, nil
}

func (m *Forger) createFeeTx(txarr []*prototype.Transaction) (*prototype.Transaction, error) {
	var feeAmount uint64

	for _, tx := range txarr {
		feeAmount += tx.GetRealFee()
	}

	if feeAmount == 0 {
		return nil, ErrZeroFeeAmount
	}

	opts := types.TxOptions{
		Outputs: []*prototype.TxOutput{
			types.NewOutput(common.HexToAddress(common.BlackHoleAddress), feeAmount, nil),
		},
		Type: common.FeeTxType,
		Fee:  0,
		Num:  m.bf.GetBlockCount(),
	}

	ntx, err := types.NewPbTransaction(opts, nil)
	if err != nil {
		return nil, err
	}

	return ntx, nil
}

func (m *Forger) createRewardTxs(blockNum uint64, proposer string) ([]*prototype.Transaction, error) {
	outs := m.att.StakePool().GetRewardOutputs(proposer)
	if len(outs) == 0 {
		return nil, consensus.ErrNoStakers
	}

	transactions := make([]*prototype.Transaction, 0)
	count := 0

	opts := types.TxOptions{
		Outputs: make([]*prototype.TxOutput, 0),
		Type:    common.RewardTxType,
		Fee:     0,
		Num:     blockNum,
	}

	for _, out := range outs {
		if count >= OutputsPerTxLimit {
			ntx, err := types.NewPbTransaction(opts, nil)
			if err != nil {
				return nil, err
			}
			transactions = append(transactions, ntx)

			// Clean up opts
			opts = types.TxOptions{
				Outputs: make([]*prototype.TxOutput, 0),
				Type:    common.RewardTxType,
				Fee:     0,
				Num:     blockNum,
			}
			count = 0
		}

		opts.Outputs = append(opts.Outputs, out)
		count += 1
	}

	if count > 0 {
		ntx, err := types.NewPbTransaction(opts, nil)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, ntx)
	}

	return transactions, nil
}

func (m *Forger) createCollapseTx(tx *types.Transaction, blockNum uint64, totalSize int) ([]*types.Transaction, error) {
	const CollapseOutputsNum = 100 // minimal count of UTxO to collapse address outputs

	transactions := make([]*types.Transaction, 0)
	var sizeCounter int

	from := ""
	if tx.Type() != common.RewardTxType {
		from = tx.From().Hex()
		m.skipAddr[from] = struct{}{}
	}

	for _, out := range tx.Outputs() {
		addr := out.Address().Hex()

		// skip already collapsed addresses and sender address
		if _, exists := m.skipAddr[addr]; exists {
			continue
		}

		opts := types.TxOptions{
			Inputs:  make([]*prototype.TxInput, 0, len(tx.Inputs())),
			Outputs: make([]*prototype.TxOutput, 0, len(tx.Outputs())-1),
			Fee:     0,
			Num:     blockNum,
			Type:    common.CollapseTxType,
		}

		utxo, err := m.bf.FindAllUTxO(addr)
		if err != nil {
			return nil, err
		}

		utxoCount := len(utxo)
		m.skipAddr[addr] = struct{}{}

		// check address need collapsing
		// should revert this
		if utxoCount < CollapseOutputsNum {
			continue
		}

		// process address outputs
		userInputs := make([]*prototype.TxInput, 0, len(utxo))
		var balance uint64
		for _, uo := range utxo {
			if len(opts.Inputs)+len(userInputs)+1 == InputsPerTxLimit {
				break
			}
			input := uo.ToPbInput()
			balance += uo.Amount
			userInputs = append(userInputs, input)

		}

		if len(userInputs) > 0 {
			opts.Inputs = append(opts.Inputs, userInputs...)
			collapsedOutput := types.NewOutput(out.Address(), balance, nil)
			opts.Outputs = append(opts.Outputs, collapsedOutput)
		}

		var collapseTx *prototype.Transaction

		if len(opts.Inputs) > 0 {
			collapseTx, err = types.NewPbTransaction(opts, nil)
			if err != nil {
				return nil, err
			}
			if collapseTx.SizeSSZ()+sizeCounter+totalSize >= m.cfg.BlockSize {
				return transactions, nil
			}

			transactions = append(transactions, types.NewTransaction(collapseTx))
			sizeCounter += collapseTx.SizeSSZ()

			log.Debugf("Collapse tx for %s with hash %s", tx.Hash().Hex(), common.Encode(collapseTx.Hash))
			if len(transactions) == collapseTxLimit {
				break
			}
		}

	}

	return transactions, nil
}

// Generates system unstake transactions for all electors of node.
func (m *Forger) generateSystemUnstakes(validator string, blockNum uint64) ([]*prototype.Transaction, error) {
	unstakeTxs := make([]*prototype.Transaction, 0)
	inputs := make([]*prototype.TxInput, 0)
	outputs := make([]*prototype.TxOutput, 0)
	electors, err := m.att.StakePool().GetElectorsOfValidator(validator)
	if err != nil {
		return nil, err
	}

	for elector := range electors {
		deposits, err := m.bf.FindStakeDepositsOfAddress(elector, validator)
		if err != nil {
			return nil, errors.Errorf("Could not get stake deposits of: %s", elector)
		}

		// The transaction is filled up, add it to the unstakeTxs
		// and proceed to the next one
		if len(deposits)+len(inputs) > InputsPerTxLimit {
			opts := types.TxOptions{
				Inputs:  inputs,
				Outputs: outputs,
				Fee:     0,
				Num:     blockNum,
				Type:    common.ValidatorsUnstakeTxType,
			}

			ntx, err := types.NewPbTransaction(opts, nil)
			if err != nil {
				return nil, err
			}
			unstakeTxs = append(unstakeTxs, ntx)

			// Reset inputs and outputs lists
			inputs = make([]*prototype.TxInput, 0)
			outputs = make([]*prototype.TxOutput, 0)
		}

		// Convert utxos to inputs and create a corrresponding output
		var amount uint64
		for _, deposit := range deposits {
			inputs = append(inputs, deposit.ToPbInput())
			amount += deposit.Amount
		}
		output := types.NewOutput(common.FromHex(elector), amount, nil)
		outputs = append(outputs, output)
	}
	if len(inputs) > 0 {
		opts := types.TxOptions{
			Inputs:  inputs,
			Outputs: outputs,
			Fee:     0,
			Num:     blockNum,
			Type:    common.ValidatorsUnstakeTxType,
		}

		ntx, err := types.NewPbTransaction(opts, nil)
		if err != nil {
			return nil, err
		}
		unstakeTxs = append(unstakeTxs, ntx)
	}

	return unstakeTxs, nil
}

func countTXsSize(txs []*prototype.Transaction) int {
	size := 0
	for _, tx := range txs {
		size += tx.SizeSSZ()
	}
	return size
}

func isBadTransaction(tx *prototype.Transaction, fullUnstakes map[string]struct{}) bool {
	// scanning through outputs, checking if node is in fullUnstakes
	if tx.Type == common.StakeTxType || tx.Type == common.UnstakeTxType {
		for _, out := range tx.Outputs {
			if _, exists := fullUnstakes[common.Encode(out.Node)]; exists {
				return true
			}
		}
	}
	return false
}
