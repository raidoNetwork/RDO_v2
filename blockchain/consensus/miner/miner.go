package miner

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"time"
)

var log = logrus.WithField("prefix", "Miner")

var ErrZeroFeeAmount = errors.New("Block has no transactions with fee.")

const txBatchLimit = 1000

// Config miner options
type Config struct {
	EnableMetrics bool
	BlockSize     int
	Proposer 	 *keystore.ValidatorAccount
}

func NewMiner(bc consensus.BlockForger, att consensus.AttestationPool, cfg *Config) *Miner {
	m := Miner{
		bc:       bc,
		att:      att,
		cfg:      cfg,
		skipAddr: map[string]struct{}{},
	}

	return &m
}

type Miner struct {
	bc   consensus.BlockForger
	att  consensus.AttestationPool
	cfg      *Config
	skipAddr map[string]struct{}
}

func (m *Miner) addRewardTxToBatch(txBatch []*prototype.Transaction, collapseBatch []*prototype.Transaction, totalSize int, bn int64) ([]*prototype.Transaction, []*prototype.Transaction, int, error) {
	rewardTx, err := m.createRewardTx(m.bc.GetBlockCount())
	if err != nil {
		if errors.Is(err, consensus.ErrNoStakers) {
			log.Warn("No stakers on current block.")
		} else {
			return nil, nil, 0, err
		}
	} else {
		txBatch = append(txBatch, rewardTx)
		totalSize += rewardTx.SizeSSZ()

		log.Debugf("Add RewardTx %s to the block", common.Encode(rewardTx.Hash))

		collapseTx, err := m.createCollapseTx(rewardTx, m.bc.GetBlockCount())
		if err != nil {
			log.Errorf("Can't collapse reward tx %s outputs", common.Encode(rewardTx.Hash))
			return nil, nil, 0, errors.Wrap(err, "Error collapsing tx outputs")
		}

		if collapseTx != nil {
			collapseBatch = append(collapseBatch, collapseTx)
			totalSize += collapseTx.SizeSSZ()

			log.Debugf("Add CollapseTx %s to the block %d", common.Encode(collapseTx.Hash), bn)
		}
	}

	return txBatch, collapseBatch, totalSize, nil
}

// ForgeBlock create block from tx pool data
func (m *Miner) ForgeBlock() (*prototype.Block, error) {
	start := time.Now()
	bn := start.Unix()
	totalSize := 0 // current size of block in bytes

	txQueue := m.att.TxPool().GetTxQueue()
	txQueueLen := len(txQueue)
	txBatch := make([]*prototype.Transaction, 0, txQueueLen)
	collapseBatch := make([]*prototype.Transaction, 0, txQueueLen)

	pendingTxCounter.Set(float64(txQueueLen))

	// create reward transaction for current block
	txBatch, collapseBatch, totalSize, err := m.addRewardTxToBatch(txBatch, collapseBatch, totalSize, bn)
	if err != nil {
		return nil, err
	}

	// limit tx count in block according to marshaller settings
	txCountPerBlock := txBatchLimit - len(txBatch) - len(collapseBatch) - 1
	if txQueueLen < txCountPerBlock {
		txCountPerBlock = txQueueLen
	}

	var size int
	for i := 0; i < txQueueLen; i++ {
		if len(txBatch) + len(collapseBatch) == txCountPerBlock {
			break
		}

		size = txQueue[i].Size()
		totalSize += size

		// tx is too big try for look up another one
		if totalSize > m.cfg.BlockSize {
			totalSize -= size
			continue
		}

		tx := txQueue[i].GetTx()
		hash := common.Encode(tx.Hash)

		// check if empty validator slots exists and skip stake tx if not exist
		if tx.Type == common.StakeTxType {
			var amount uint64
			for _, out := range tx.Outputs {
				if common.BytesToAddress(out.Node).Hex() == common.BlackHoleAddress {
					amount += out.Amount
				}
			}

			err = m.att.StakePool().ReserveSlots(amount)
			if err != nil {
				totalSize -= size // return size of current tx

				log.Warnf("Skip stake tx %s: %s", hash, err)

				// Delete stake tx from pool
				err = m.att.TxPool().DeleteTransaction(tx)
				if err != nil {
					m.att.TxPool().RollbackReserved()
					return nil, errors.Wrap(err, "Error creating block")
				}

				continue
			}

			log.Debugf("Add stake tx %s to the block.", hash)
		}

		txBatch = append(txBatch, tx)
		log.Debugf("Add PoolTx %s to the block %d", hash, bn)

		// reserve tx for block forging
		err = m.att.TxPool().ReserveTransaction(tx)
		if err != nil {
			m.att.TxPool().RollbackReserved()
			return nil, err
		}

		collapseTx, err := m.createCollapseTx(tx, m.bc.GetBlockCount())
		if err != nil {
			m.att.TxPool().RollbackReserved()
			log.Errorf("Can't collapse tx %s outputs", hash)
			return nil, errors.Wrap(err, "Error collapsing tx outputs")
		}

		// no need to create collapse tx for given tx addresses
		if collapseTx != nil {
			collapseBatch = append(collapseBatch, collapseTx)
			totalSize += collapseTx.SizeSSZ()

			log.Debugf("Add CollapseTx %s to the block %d", common.Encode(collapseTx.Hash), bn)
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
			if errors.Is(err, ErrZeroFeeAmount) {
				log.Debug(err)
			} else {
				return nil, err
			}
		} else {
			txBatch = append(txBatch, txFee)

			log.Debugf("Add FeeTx %s to the block", common.Encode(txFee.Hash))
		}
	}

	// clear collapse list
	m.skipAddr = map[string]struct{}{}

	// get block instance
	block := m.generateBlockBody(txBatch)

	end := time.Since(start)
	log.Warnf("Generate block with transactions count: %d. TxPool transactions count: %d. Size: %d kB. Time: %s", len(txBatch), txQueueLen, totalSize/1024, common.StatFmt(end))
	updateForgeMetrics(end)

	return block, nil
}

// FinalizeBlock validate given block and save it to the blockchain
func (m *Miner) FinalizeBlock(block *prototype.Block) error {
	finalizeStart := time.Now()
	start := time.Now()

	err := m.att.TxPool().ReserveTransactions(block.Transactions)
	if err != nil {
		m.att.TxPool().RollbackReserved()
		return errors.Wrap(err, "Error reserving transactions")
	}

	// validate block
	err = m.att.Validator().ValidateBlock(block)
	if err != nil {
		m.att.TxPool().RollbackReserved()
		return errors.Wrap(err, "ValidateBlockError")
	}

	if m.cfg.EnableMetrics {
		end := time.Since(start)
		log.Infof("FinalizeBlock: Validate block in %s", common.StatFmt(end))
	}

	start = time.Now()

	// save block
	err = m.bc.SaveBlock(block)
	if err != nil {
		m.att.TxPool().RollbackReserved()
		return err
	}

	if m.cfg.EnableMetrics {
		end := time.Since(start)
		log.Infof("FinalizeBlock: Store block in %s", common.StatFmt(end))
	}

	// update SQL
	err = m.bc.ProcessBlock(block)
	if err != nil {
		return errors.Wrap(err, "Error process block")
	}

	startInner := time.Now()

	err = m.att.StakePool().UpdateStakeSlots(block)
	if err != nil {
		return errors.Wrap(err, "StakePool error")
	}

	// Reset reserved validator slots
	m.att.StakePool().FlushReservedSlots()

	if m.cfg.EnableMetrics {
		end := time.Since(startInner)
		log.Infof("FinalizeBlock: Update stake slots in %s", common.StatFmt(end))
	}

	startInner = time.Now()

	// Reset reserved pool and clean all extra staking
	m.att.TxPool().FlushReserved(true)

	if m.cfg.EnableMetrics {
		endInner := time.Since(startInner)
		log.Infof("FinalizeBlock: Clean transaction pool in %s", common.StatFmt(endInner))
	}

	err = m.bc.CheckBalance()
	if err != nil {
		return errors.Wrap(err, "Balances inconsistency")
	}

	finalizeBlockTime.Observe(float64(time.Since(finalizeStart).Milliseconds()))

	return nil
}

// generateBlockBody creates block from given batch of transactions and store it to the database.
func (m *Miner) generateBlockBody(txBatch []*prototype.Transaction) *prototype.Block {
	return types.NewBlock(m.bc.GetBlockCount(), m.bc.ParentHash(), txBatch, m.cfg.Proposer)
}

func (m *Miner) createFeeTx(txarr []*prototype.Transaction) (*prototype.Transaction, error) {
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
		Num:  m.bc.GetBlockCount(),
	}

	ntx, err := types.NewTx(opts, nil)
	if err != nil {
		return nil, err
	}

	return ntx, nil
}

func (m *Miner) createRewardTx(blockNum uint64) (*prototype.Transaction, error) {
	slots := m.att.StakePool().GetStakeSlots()
	outs := m.createRewardOutputs(slots)

	if len(outs) == 0 {
		return nil, consensus.ErrNoStakers
	}

	opts := types.TxOptions{
		Outputs: outs,
		Type:    common.RewardTxType,
		Fee:     0,
		Num:     blockNum,
	}

	ntx, err := types.NewTx(opts, nil)
	if err != nil {
		return nil, err
	}

	return ntx, nil
}

func (m *Miner) createCollapseTx(tx *prototype.Transaction, blockNum uint64) (*prototype.Transaction, error) {
	const CollapseOutputsNum = 100 // minimal count of UTxO to collapse address outputs
	const InputsPerTxLimit = 2000

	opts := types.TxOptions{
		Inputs: make([]*prototype.TxInput, 0, len(tx.Inputs)),
		Outputs: make([]*prototype.TxOutput, 0, len(tx.Outputs) - 1),
		Fee: 0,
		Num: blockNum,
		Type: common.CollapseTxType,
	}

	from := ""
	if tx.Type != common.RewardTxType {
		from = common.BytesToAddress(tx.Inputs[0].Address).Hex()
		m.skipAddr[from] = struct{}{}
	}

	inputsLimitReached := false
	for _, out := range tx.Outputs {
		addr := common.BytesToAddress(out.Address).Hex()

		// skip already collapsed addresses and sender address
		if _, exists := m.skipAddr[addr]; exists {
			continue
		}

		utxo, err := m.bc.FindAllUTxO(addr)
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
		var balance uint64
		for _, uo := range utxo {
			input := uo.ToInput()

			if m.att.TxPool().IsLockedInput(input) {
				// do not create tx for address with locked inputs
				return nil, nil
			}

			balance += uo.Amount
			opts.Inputs = append(opts.Inputs, input)

			if len(opts.Inputs) == InputsPerTxLimit {
				inputsLimitReached = true
				break
			}
		}

		// add new output for address
		collapsedOutput := types.NewOutput(out.Address, balance, nil)
		opts.Outputs = append(opts.Outputs, collapsedOutput)

		// break generation when inputs limit is reached
		if inputsLimitReached {
			break
		}
	}

	var collapseTx *prototype.Transaction
	var err error

	if len(opts.Inputs) > 0 {
		collapseTx, err = types.NewTx(opts, nil)
		if err != nil {
			return nil, err
		}

		log.Debugf("Collapse tx for %s with hash %s", common.Encode(tx.Hash), common.Encode(collapseTx.Hash))
	}

	return collapseTx, nil
}

func (m *Miner) createRewardOutputs(slots []string) []*prototype.TxOutput {
	size := len(slots)

	data := make([]*prototype.TxOutput, 0, size)
	if size == 0 {
		return data
	}

	// divide reward among all validator slots
	reward := m.getRewardAmount(size)

	rewardMap := map[string]uint64{}
	for _, addrHex := range slots {
		if _, exists := rewardMap[addrHex]; exists {
			rewardMap[addrHex] += reward
		} else {
			rewardMap[addrHex] = reward
		}
	}

	for addr, amount := range rewardMap {
		addr := common.HexToAddress(addr)
		data = append(data, types.NewOutput(addr.Bytes(), amount, nil))
	}

	return data
}

func (m *Miner) getRewardAmount(size int) uint64 {
	if size == 0 {
		return 0
	}

	return params.RaidoConfig().RewardBase / uint64(size)
}