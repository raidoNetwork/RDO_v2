package forger

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"time"
)

var log = logrus.WithField("prefix", "Forger")

var (
	ErrZeroFeeAmount = errors.New("Block has no transactions with fee.")
)

const txBatchLimit = 1000

type Config struct {
	EnableMetrics bool
	BlockSize     int
	Proposer 	 *keystore.ValidatorAccount
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
	bf  consensus.BlockFinalizer
	att consensus.AttestationPool
	cfg      *Config
	skipAddr map[string]struct{}
}

// ForgeBlock create block from tx pool data
func (m *Forger) ForgeBlock() (*prototype.Block, error) {
	start := time.Now()
	bn := start.Unix()
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

	pendingTxCounter.Set(float64(txQueueLen)) // todo remove metrics to the core service

	// create reward transaction for current block
	txBatch, collapseBatch, totalSize, err := m.addRewardTxToBatch(m.cfg.Proposer.Addr().Hex(), txBatch, collapseBatch, totalSize, bn)
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
	for _, tx := range txQueue {
		if len(txBatch) + len(collapseBatch) == txCountPerBlock {
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
			continue
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
					amount += out.Amount()
				}
			}

			if amount > 0 {
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

		txBatch = append(txBatch, tx.GetTx())
		log.Debugf("Add %s %s to the block %d", txType, hash, bn)
		txType = "StandardTx"

		collapseTx, err := m.createCollapseTx(tx, m.bf.GetBlockCount())
		if err != nil {
			log.Errorf("Can't collapse tx %s outputs", hash)
			return nil, errors.Wrap(err, "Error collapsing tx outputs")
		}

		// no need to create collapse tx for given tx addresses
		if collapseTx != nil {
			err := m.att.TxPool().InsertCollapseTx(collapseTx)
			if err != nil {
				log.Error(err)
			} else {
				collapseBatch = append(collapseBatch, collapseTx.GetTx())
				totalSize += collapseTx.Size()

				log.Debugf("Add CollapseTx %s to the block %d", collapseTx.Hash().Hex(), bn)
			}
		}

		// we fill block successfully
		if totalSize == m.cfg.BlockSize {
			break
		}
	}

	// add collapsed transaction to the end of the batch
	txBatch = append(txBatch, collapseBatch...)

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

func (m *Forger) addRewardTxToBatch(proposer string, txBatch []*prototype.Transaction, collapseBatch []*prototype.Transaction, totalSize int, bn int64) ([]*prototype.Transaction, []*prototype.Transaction, int, error) {
	rewardTx, err := m.createRewardTx(m.bf.GetBlockCount(), proposer)
	if err != nil {
		if errors.Is(err, consensus.ErrNoStakers) {
			log.Debug(err)
		} else {
			return nil, nil, 0, err
		}
	} else {
		txBatch = append(txBatch, rewardTx)
		totalSize += rewardTx.SizeSSZ()

		log.Debugf("Add RewardTx %s to the block", common.Encode(rewardTx.Hash))

		collapseTx, err := m.createCollapseTx(types.NewTransaction(rewardTx), m.bf.GetBlockCount())
		if err != nil {
			log.Errorf("Can't collapse reward tx %s outputs", common.Encode(rewardTx.Hash))
			return nil, nil, 0, errors.Wrap(err, "Error collapsing tx outputs")
		}

		if collapseTx != nil {
			err := m.att.TxPool().InsertCollapseTx(collapseTx)
			if err != nil {
				return txBatch, collapseBatch, totalSize, nil
			}

			collapseBatch = append(collapseBatch, collapseTx.GetTx())
			totalSize += collapseTx.Size()

			log.Debugf("Add CollapseTx %s to the block %d", collapseTx.Hash().Hex(), bn)
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
			types.NewOutput(common.HexToAddress(common.BlackHoleAddress).Bytes(), feeAmount, nil),
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

func (m *Forger) createRewardTx(blockNum uint64, proposer string) (*prototype.Transaction, error) {
	outs := m.att.StakePool().GetRewardOutputs(proposer)
	if len(outs) == 0 {
		return nil, consensus.ErrNoStakers
	}

	opts := types.TxOptions{
		Outputs: outs,
		Type:    common.RewardTxType,
		Fee:     0,
		Num:     blockNum,
	}

	ntx, err := types.NewPbTransaction(opts, nil)
	if err != nil {
		return nil, err
	}

	return ntx, nil
}

func (m *Forger) createCollapseTx(tx *types.Transaction, blockNum uint64) (*types.Transaction, error) {
	const CollapseOutputsNum = 100 // minimal count of UTxO to collapse address outputs
	const InputsPerTxLimit = 2000

	opts := types.TxOptions{
		Inputs: make([]*prototype.TxInput, 0, len(tx.Inputs())),
		Outputs: make([]*prototype.TxOutput, 0, len(tx.Outputs()) - 1),
		Fee: 0,
		Num: blockNum,
		Type: common.CollapseTxType,
	}

	from := ""
	if tx.Type() != common.RewardTxType {
		from = tx.From().Hex()
		m.skipAddr[from] = struct{}{}
	}

	inputsLimitReached := false
	for _, out := range tx.Outputs() {
		addr := out.Address().Hex()

		// skip already collapsed addresses and sender address
		if _, exists := m.skipAddr[addr]; exists {
			continue
		}

		utxo, err := m.bf.FindAllUTxO(addr)
		if err != nil {
			return nil, err
		}

		utxoCount := len(utxo)
		m.skipAddr[addr] = struct{}{}

		// check address need collapsing
		if utxoCount < CollapseOutputsNum {
			continue
		}

		// process address outputs
		userInputs := make([]*prototype.TxInput, 0, len(utxo))
		var balance uint64
		for _, uo := range utxo {
			input := uo.ToPbInput()

			balance += uo.Amount
			userInputs = append(userInputs, input)

			if len(opts.Inputs) + len(userInputs) == InputsPerTxLimit {
				inputsLimitReached = true
				break
			}
		}

		if len(userInputs) > 0 {
			opts.Inputs = append(opts.Inputs, userInputs...)
			collapsedOutput := types.NewOutput(out.Address(), balance, nil)
			opts.Outputs = append(opts.Outputs, collapsedOutput)
		}

		// break generation when inputs limit is reached
		if inputsLimitReached {
			break
		}
	}

	var collapseTx *prototype.Transaction
	var err error

	if len(opts.Inputs) > 0 {
		collapseTx, err = types.NewPbTransaction(opts, nil)
		if err != nil {
			return nil, err
		}

		log.Debugf("Collapse tx for %s with hash %s", tx.Hash().Hex(), common.Encode(collapseTx.Hash))
	}

	return types.NewTransaction(collapseTx), nil
}