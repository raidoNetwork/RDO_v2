package miner

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/hashutil"
	"github.com/raidoNetwork/RDO_v2/shared/keystore"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "Miner")

var ErrZeroFeeAmount = errors.New("Block has no transactions with fee.")

// MinerConfig miner options
type MinerConfig struct {
	ShowStat     bool
	ShowFullStat bool
	BlockSize    int
	Proposer 	 *keystore.ValidatorAccount
}

func NewMiner(bc consensus.BlockForger, att consensus.AttestationPool, cfg *MinerConfig) *Miner {
	m := Miner{
		bc:  bc,
		att: att,
		cfg: cfg,
		collapsedAddr: map[string]int{},
		signer: types.MakeBlockSigner("keccak256"),
	}

	return &m
}

type Miner struct {
	bc   consensus.BlockForger
	att  consensus.AttestationPool

	signer types.BlockSigner

	cfg *MinerConfig
	mu  sync.RWMutex

	collapsedAddr map[string]int
}

// GenerateBlock create block from tx pool data
func (m *Miner) GenerateBlock() (*prototype.Block, error) {
	start := time.Now()
	totalSize := 0 // current size of block in bytes

	txList := m.att.TxPool().GetTxQueue()
	txListLen := len(txList)
	txBatch := make([]*prototype.Transaction, 0, txListLen)
	collapseBatch := make([]*prototype.Transaction, 0, 4)

	// create reward transaction for current block
	rewardTx, err := m.createRewardTx(m.bc.GetBlockCount())
	if err != nil {
		if errors.Is(err, consensus.ErrNoStakers) {
			log.Warn("No stakers on current block.")
		} else {
			return nil, err
		}
	} else {
		txBatch = append(txBatch, rewardTx)
		totalSize += rewardTx.SizeSSZ()

		log.Warnf("Add RewardTx %s to the block", common.Encode(rewardTx.Hash))

		collapseTx, err := m.createCollapseTx(rewardTx, m.bc.GetBlockCount())
		if err != nil {
			log.Errorf("Can't collapse reward tx %s outputs", common.Encode(rewardTx.Hash))
			return nil, errors.Wrap(err, "Error collapsing tx outputs")
		}

		if collapseTx != nil {
			collapseBatch = append(collapseBatch, collapseTx)
			totalSize += collapseTx.SizeSSZ()

			log.Warnf("Add CollapseTx %s to the block", common.Encode(collapseTx.Hash))
		}
	}

	// limit tx count in block according to marshaller settings
	txBatchLimit := 1000
	if txListLen < txBatchLimit {
		txBatchLimit = txListLen
	}

	var size int
	for i := 0; i < txBatchLimit; i++ {
		size = txList[i].Size()
		totalSize += size

		// tx is too big try for look up another one
		if totalSize > m.cfg.BlockSize {
			totalSize -= size
			continue
		}

		tx := txList[i].GetTx()
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
					return nil, errors.Wrap(err, "Error creating block")
				}

				continue
			}

			log.Infof("Add stake tx %s to the block.", hash)
		}

		txBatch = append(txBatch, tx)

		collapseTx, err := m.createCollapseTx(tx, m.bc.GetBlockCount())
		if err != nil {
			log.Errorf("Can't collapse tx %s outputs", hash)
			return nil, errors.Wrap(err, "Error collapsing tx outputs")
		}

		// no need to create collapse tx for given tx addresses
		if collapseTx != nil {
			collapseBatch = append(collapseBatch, collapseTx)
			totalSize += collapseTx.SizeSSZ()

			log.Warnf("Add CollapseTx %s to the block", common.Encode(collapseTx.Hash))
		}

		// we fill block successfully
		if totalSize == m.cfg.BlockSize {
			break
		}
	}

	// set given tx as received in order to delete them from pool
	if txListLen > 0 {
		err = m.att.TxPool().ReserveTransactions(txBatch)
		if err != nil {
			return nil, err
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
			} else {
				log.Debug(err)
			}
		} else {
			txBatch = append(txBatch, txFee)

			log.Warnf("Add FeeTx %s to the block", common.Encode(txFee.Hash))
		}
	}

	// get block instance
	block := m.generateBlockBody(txBatch)

	end := time.Since(start)
	log.Warnf("Generate block with transactions count: %d. TxPool transactions count: %d. Size: %d kB. Time: %s", len(txBatch), txListLen, totalSize/1024, common.StatFmt(end))

	return block, nil
}

// FinalizeBlock validate given block and save it to the blockchain
func (m *Miner) FinalizeBlock(block *prototype.Block) error {
	start := time.Now()

	// validate block
	err := m.att.Validator().ValidateBlock(block)
	if err != nil {
		m.att.TxPool().RollbackReserved()
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
		m.att.TxPool().RollbackReserved()
		return err
	}

	if m.cfg.ShowStat {
		end := time.Since(start)
		log.Infof("FinalizeBlock: Store block in %s", common.StatFmt(end))
	}

	// clear collapse list
	m.collapsedAddr = map[string]int{}

	// update SQL
	err = m.bc.ProcessBlock(block) // has statistic inside
	if err != nil {
		log.Errorf("FinalizeBlock: Error process block: %s", err)

		// try to resync SQL with KV
		err = m.bc.SyncData()
		if err != nil {
			return err
		}
	}

	startInner := time.Now()

	err = m.att.StakePool().UpdateStakeSlots(block)
	if err != nil {
		return errors.Wrap(err, "StakePool error")
	}

	// Reset reserved validator slots
	m.att.StakePool().FlushReservedSlots()

	if m.cfg.ShowStat {
		end := time.Since(startInner)
		log.Infof("FinalizeBlock: Update stake slots in %s", common.StatFmt(end))
	}

	startInner = time.Now()

	// Reset reserved pool and clean all extra staking
	m.att.TxPool().FlushReserved(true)

	if m.cfg.ShowStat {
		endInner := time.Since(startInner)
		log.Infof("FinalizeBlock: Clean transaction pool in %s", common.StatFmt(endInner))
	}

	return nil
}

// ForgeBlock create and save new block
func (m *Miner) ForgeBlock() (*prototype.Block, error) {
	start := time.Now()
	var end time.Duration

	// generate block with block miner
	block, err := m.GenerateBlock()
	if err != nil {
		return nil, err
	}

	if m.cfg.ShowStat {
		end = time.Since(start)
		log.Infof("ForgeBlock: Generate block in %s.", common.StatFmt(end))
	}

	start = time.Now()

	// validate, save block and update SQL
	err = m.FinalizeBlock(block)
	if err != nil {
		return nil, err
	}

	if m.cfg.ShowStat {
		end = time.Since(start)
		log.Infof("ForgeBlock: Finalize block in %s.", common.StatFmt(end))
	}

	return block, nil
}

// generateBlockBody creates block from given batch of transactions and store it to the database.
func (m *Miner) generateBlockBody(txBatch []*prototype.Transaction) *prototype.Block {
	header := &types.BlockHeader{
		Num: m.bc.GetBlockCount(),
		Parent: m.bc.ParentHash(),
		Version: []byte{1, 0, 0},
		TxRoot: hashutil.GenTxRoot(txBatch),
	}

	// sign block
	sheet := &types.BlockSheet{
		Proposer: m.signBlock(header),
	}

	return types.NewBlock(header, txBatch, sheet)
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
	const CollapseOutputsNum = 100
	const InputsPerTxLimit = 2000

	opts := types.TxOptions{
		Inputs: []*prototype.TxInput{},
		Outputs: []*prototype.TxOutput{},
		Fee: 0,
		Num: blockNum,
		Type: common.CollapseTxType,
	}

	from := ""
	if tx.Type != common.RewardTxType {
		from = common.BytesToAddress(tx.Inputs[0].Address).Hex()
	}

	for _, out := range tx.Outputs {
		addr := common.BytesToAddress(out.Address).Hex()

		// skip already collapsed addresses and sender address
		if _, exists := m.collapsedAddr[addr]; exists || addr == from {
			continue
		}

		utxo, err := m.bc.FindAllUTxO(addr)
		if err != nil {
			return nil, err
		}

		inputsCount := len(utxo)
		if inputsCount + 1 <= CollapseOutputsNum {
			continue
		}

		skipAddr := true
		inputsLimitReached := false

		var balance uint64 = 0
		storedInputs := 0
		for _, uo := range utxo {
			balance += uo.Amount
			opts.Inputs = append(opts.Inputs, uo.ToInput())

			storedInputs++

			if len(opts.Inputs) == InputsPerTxLimit {
				skipAddr = inputsCount - storedInputs > CollapseOutputsNum
				inputsLimitReached = true
				break
			}
		}

		collapsedOutput := types.NewOutput(out.Address, balance, nil)
		opts.Outputs = append(opts.Outputs, collapsedOutput)

		// if address has a lot of inputs yet and
		// block has another transaction with inputs for given address
		// skip this address for generation new collapse tx
		if skipAddr {
			m.collapsedAddr[addr] = 1
		}

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
		} else{
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

func (m *Miner) signBlock(header *types.BlockHeader) *prototype.Sign {
	sign, err := m.signer.Sign(header, m.cfg.Proposer.Key())
	if err != nil {
		panic(errors.Wrap(err, "error signing block"))
	}

	return sign
}