package rdochain

import (
	"bytes"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/kv"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
)

const (
	GenesisBlockNum = 0
	limitFetch      = 1000
)

func NewBlockChain(db db.BlockStorage, cfg *params.RDOBlockChainConfig) *BlockChain {
	genesisHash := make([]byte, 32)

	bc := BlockChain{
		db:             db,
		prevHash:       genesisHash,
		headBlockNum:   GenesisBlockNum,
		futureBlockNum: GenesisBlockNum + 1, // block num for the future block
		genesisHash:    genesisHash,
		cfg:            cfg,
	}

	return &bc
}

type BlockChain struct {
	db db.BlockStorage

	prevHash common.Hash

	futureBlockNum uint64
	headBlockNum   uint64
	headBlock      *prototype.Block
	genesisBlock   *prototype.Block
	genesisHash    common.Hash

	mu sync.Mutex

	cfg *params.RDOBlockChainConfig
}

// Init check database and update block num and previous hash
func (bc *BlockChain) Init() error {
	// get head block
	head, err := bc.db.GetHeadBlockNum()
	if err != nil {
		if errors.Is(err, kv.ErrNoHead) {
			count, err := bc.db.CountBlocks()
			if err != nil {
				return err
			}

			// save head block link
			head = uint64(count) // skip genesis key
			err = bc.saveHeadBlockNum(head)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// insert Genesis if not exists
	err = bc.insertGenesis()
	if err != nil {
		return err
	}

	// update counter according to the database
	bc.mu.Lock()
	bc.headBlockNum = head                  // head block number
	bc.futureBlockNum = bc.headBlockNum + 1 // future block num
	bc.mu.Unlock()

	var block *prototype.Block
	if head != GenesisBlockNum {
		// get last block
		block, err = bc.GetBlockByNum(bc.headBlockNum)
	} else {
		// if head is equal to zero return Genesis
		block, err = bc.db.GetGenesis()
		bc.genesisHash = block.Hash[:]
	}

	if err != nil {
		log.Errorf("Error reading head block #%d", head)
		return err
	}

	if block == nil {
		log.Errorf("Init: Not found block with num %d.", bc.headBlockNum)
		return errors.New("block not found")
	}

	log.Infof("Database has %d blocks. Future block num %d.", bc.headBlockNum, bc.futureBlockNum)

	bc.mu.Lock()
	bc.prevHash = block.Hash[:]
	bc.headBlock = block
	bc.mu.Unlock()

	// update metrics
	setStartupMetrics(bc.genesisBlock, bc.headBlockNum)

	return nil
}

// saveHeadBlockNum save given number to the database as head number.
func (bc *BlockChain) saveHeadBlockNum(n uint64) error {
	return bc.db.SaveHeadBlockNum(n)
}

// ParentHash return parent hash for current block
func (bc *BlockChain) ParentHash() []byte {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.prevHash
}

// SaveBlock stores given block in the database.
func (bc *BlockChain) SaveBlock(block *prototype.Block) error {
	start := time.Now()

	err := bc.db.WriteBlock(block)
	if err != nil {
		return errors.Wrap(err, "Error saving block to the KV")
	}

	blockSavingTime.Observe(float64(time.Since(start).Milliseconds()))

	start = time.Now()
	err = bc.saveHeadBlockNum(block.Num)
	if err != nil {
		return err
	}

	blockHeadSavingTime.Observe(float64(time.Since(start).Milliseconds()))
	start = time.Now()

	var fee, reward uint64 // fee amount for block
	for _, tx := range block.Transactions {
		if tx.Type == common.RewardTxType || tx.Type == common.FeeTxType {
			for _, out := range tx.Outputs {
				if tx.Type == common.RewardTxType {
					reward += out.Amount
				} else {
					fee += out.Amount
				}
			}
		}
	}

	err = bc.db.UpdateAmountStats(reward, fee)
	if err != nil {
		return err
	}

	supplySavingTime.Observe(float64(time.Since(start).Milliseconds()))

	// update blocks stats
	bc.mu.Lock()
	bc.headBlockNum++
	bc.futureBlockNum++
	bc.prevHash = block.Hash
	bc.headBlock = block
	bc.mu.Unlock()

	headBlockNum.Inc()

	return nil
}

// GetBlockByNum returns block from database by block number
func (bc *BlockChain) GetBlockByNum(num uint64) (*prototype.Block, error) {
	if num == 0 {
		return bc.GetGenesis(), nil
	}

	if num > bc.GetHeadBlockNum() {
		return nil, ErrNotForgedBlock
	}

	return bc.db.GetBlockByNum(num)
}

// GetBlockByHash returns block
func (bc *BlockChain) GetBlockByHash(hash []byte) (*prototype.Block, error) {
	if bytes.Equal(hash, bc.genesisHash) {
		return bc.GetGenesis(), nil
	}

	return bc.db.GetBlockByHash(hash)
}

// GetBlockCount return block count
func (bc *BlockChain) GetBlockCount() uint64 {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.futureBlockNum
}

// GetHeadBlock get last block
func (bc *BlockChain) GetHeadBlock() (*prototype.Block, error) {
	bc.mu.Lock()
	headBlock := bc.headBlock
	bc.mu.Unlock()

	if headBlock == nil {
		blk, err := bc.GetBlockByNum(bc.headBlockNum)
		if err != nil {
			return nil, err
		}

		if blk == nil {
			return nil, errors.Errorf("GetCurrentBlock: Not found block with number %d.", bc.headBlockNum)
		}

		bc.mu.Lock()
		defer bc.mu.Unlock()

		bc.headBlock = blk
		return blk, nil
	}

	return headBlock, nil
}

// GetHeadBlockNum get number of last block in the chain
func (bc *BlockChain) GetHeadBlockNum() uint64 {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.headBlockNum
}

// GetTransaction get transaction with given hash from KV.
func (bc *BlockChain) GetTransaction(hash string) (*prototype.Transaction, error) {
	tx, err := bc.db.GetTransactionByHash(common.HexToHash(hash).Bytes())
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// GetTransactionsCount get address nonce.
func (bc *BlockChain) GetTransactionsCount(addr []byte) (uint64, error) {
	return bc.db.GetTransactionsCount(addr)
}

// GetAmountStats returns total reward and fee amount
func (bc *BlockChain) GetAmountStats() (uint64, uint64, uint64) {
	bc.mu.Lock()
	genesis := bc.genesisBlock
	bc.mu.Unlock()

	if genesis == nil {
		return 0, 0, 0
	}

	reward, fee := bc.db.GetAmountStats()

	var genesisAmount uint64
	for _, tx := range genesis.Transactions {
		for _, out := range tx.Outputs {
			genesisAmount += out.Amount
		}
	}

	return reward, fee, genesisAmount
}

// GetBlockBySlot returns block associated with given slot
func (bc *BlockChain) GetBlockBySlot(slot uint64) (*prototype.Block, error) {
	return bc.db.GetBlockBySlot(slot)
}

// GetBlocksStartCount returns a number of blocks from start
// Negative start value signifies backward direction
func (bc *BlockChain) GetBlocksStartCount(start int64, limit uint32) ([]*prototype.Block, error) {
	if start > 0 && uint64(start) > bc.GetHeadBlockNum() {
		return nil, errors.New("start index is too far off")
	}

	if start < 0 && uint64(-start) > bc.GetHeadBlockNum() {
		return nil, errors.New("start index is too far off")
	}

	if limit > limitFetch {
		return nil, errors.New("the limit is too large")
	}

	res := make([]*prototype.Block, 0)

	var index uint64

	if start < 0 {
		start = -start
		// starting from the back
		index = bc.GetHeadBlockNum() - uint64(start) + 1
	} else {
		index = uint64(start)
	}

	var i uint32
	for i = 0; i < limit; i++ {
		block, err := bc.GetBlockByNum(index)
		if err == ErrNotForgedBlock {
			break
		}
		res = append(res, block)
		index += 1
	}
	return res, nil
}
