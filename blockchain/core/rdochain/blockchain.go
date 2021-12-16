package rdochain

import (
	"bytes"
	"crypto/rand"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/db"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/kv"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/hasher"
	"github.com/raidoNetwork/RDO_v2/shared/hashutil"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"sync"
	"time"
)

const (
	GenesisBlockNum = 0
)

var log = logrus.WithField("prefix", "rdochain")

var (
	randSignArr = make([]byte, crypto.SignatureLength)
	nodeAddress = crypto.Keccak256Hash([]byte("super-node"))[12:]
)

func init() {
	// create random sign hash for block validators
	rand.Read(randSignArr)
}

func NewBlockChain(db db.BlockStorage, ctx *cli.Context, cfg *params.RDOBlockChainConfig) (*BlockChain, error) {
	genesisHash := crypto.Keccak256Hash([]byte("genesis"))

	bc := BlockChain{
		db:             db,
		prevHash:       genesisHash,
		headBlockNum:   GenesisBlockNum,
		futureBlockNum: GenesisBlockNum + 1,          // block num for the future block
		showTimeStat:   ctx.Bool(flags.SrvStat.Name), // stat flag
		genesisHash:    genesisHash,
		lock:           sync.RWMutex{},
		cfg:            cfg,
	}

	err := bc.Init()
	if err != nil {
		return nil, err
	}

	return &bc, nil
}

type BlockChain struct {
	db db.BlockStorage

	prevHash     []byte
	showTimeStat bool

	futureBlockNum uint64
	headBlockNum   uint64
	headBlock      *prototype.Block
	genesisBlock   *prototype.Block
	genesisHash    []byte

	lock sync.RWMutex

	cfg *params.RDOBlockChainConfig
}

// Init check database and update block num and previous hash
func (bc *BlockChain) Init() error {
	log.Info("Init blockchain data.")

	// get head block
	head, err := bc.db.GetHeadBlockNum()
	if err != nil {
		if errors.Is(err, kv.ErrNoHead) {
			start := time.Now()

			count, err := bc.db.CountBlocks()
			if err != nil {
				return err
			}

			if bc.showTimeStat {
				end := time.Since(start)
				log.Infof("Init: Count KV rows num in %s.", common.StatFmt(end))
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

	var block *prototype.Block

	// update counter according to the database
	bc.headBlockNum = head                  // head block number
	bc.futureBlockNum = bc.headBlockNum + 1 // future block num

	if head != 0 {
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

	bc.prevHash = block.Hash[:]
	bc.headBlock = block

	return nil
}

// GenerateBlock creates block from given batch of transactions and store it to the database.
func (bc *BlockChain) GenerateBlock(txBatch []*prototype.Transaction) (*prototype.Block, error) {
	// generate fee tx for block
	if len(txBatch) > 0 {
		txFee, err := bc.createFeeTx(txBatch)
		if err != nil {
			if !errors.Is(err, ErrZeroFeeAmount) {
				return nil, err
			} else {
				log.Debug(err)
			}
		} else {
			txBatch = append(txBatch, txFee)
		}
	}

	start := time.Now()

	// create tx merklee tree root
	txRoot, err := bc.GenTxRoot(txBatch)
	if err != nil {
		return nil, err
	}

	if bc.showTimeStat {
		end := time.Since(start)
		log.Debugf("GenerateBlock: Create tx root in %s.", common.StatFmt(end))
	}

	// get num for future block
	blockNum := bc.GetBlockCount()

	version := []byte{1, 0, 0}
	tstamp := uint64(time.Now().UnixNano())

	// generate block hash
	hash := hasher.BlockHash(blockNum, version, bc.prevHash, txRoot, tstamp, nodeAddress.Bytes())

	block := &prototype.Block{
		Num:          blockNum,
		Version:      version,
		Hash:         hash,
		Parent:       bc.prevHash,
		Txroot:       txRoot,
		Timestamp:    tstamp,
		Proposer:     bc.sign(nodeAddress.Bytes()),
		Transactions: txBatch,
	}

	return block, nil
}

// saveHeadBlockNum save given number to the database as head number.
func (bc *BlockChain) saveHeadBlockNum(n uint64) error {
	return bc.db.SaveHeadBlockNum(n)
}

// SaveBlock stores given block in the database.
func (bc *BlockChain) SaveBlock(block *prototype.Block) error {
	var end time.Duration
	start := time.Now()

	err := bc.db.WriteBlock(block)
	if err != nil {
		return err
	}

	if bc.showTimeStat {
		end = time.Since(start)
		log.Infof("Save block data in %s", common.StatFmt(end))
	}

	start = time.Now()

	err = bc.saveHeadBlockNum(block.Num)
	if err != nil {
		return err
	}

	if bc.showTimeStat {
		end = time.Since(start)
		log.Infof("Save head block number in %s", common.StatFmt(end))
	}

	start = time.Now()

	var fee, reward uint64 // fee amount for block
	for _, tx := range block.Transactions {
		if tx.Type == common.RewardTxType {
			reward = tx.Outputs[0].Amount * uint64(len(tx.Outputs))
		}

		if tx.Type == common.FeeTxType {
			for _, out := range tx.Outputs {
				fee += out.Amount
			}
		}
	}

	err = bc.db.UpdateAmountStats(reward, fee)
	if err != nil {
		return err
	}

	if bc.showTimeStat {
		end = time.Since(start)
		log.Infof("Save supply data in %s", common.StatFmt(end))
	}

	// update blocks stats
	bc.lock.Lock()
	bc.headBlockNum++
	bc.futureBlockNum++
	bc.prevHash = block.Hash
	bc.headBlock = block
	bc.lock.Unlock()

	return nil
}

// GetBlockByNum returns block from database by block number
func (bc *BlockChain) GetBlockByNum(num uint64) (*prototype.Block, error) {
	if num == 0 {
		return bc.GetGenesis(), nil
	}

	if num > bc.GetHeadBlockNum() {
		return nil, errors.New("Given block number is not forged yet.")
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

// sign test function for signing some data
func (bc *BlockChain) sign(addr []byte) *prototype.Sign {
	if len(addr) > common.AddressLength {
		addr = addr[:common.AddressLength]
	}

	sign := &prototype.Sign{
		Address:   addr,
		Signature: randSignArr,
	}

	return sign
}

// GetBlockCount return block count
func (bc *BlockChain) GetBlockCount() uint64 {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	return bc.futureBlockNum
}

// GenTxRoot create transactions root hash of block
func (bc *BlockChain) GenTxRoot(txarr []*prototype.Transaction) ([]byte, error) {
	data := make([][]byte, 0, len(txarr))

	for _, tx := range txarr {
		data = append(data, tx.Hash)
	}

	root, err := hashutil.MerkleeRoot(data)
	if err != nil {
		return nil, err
	}

	return root, nil
}

// GetHeadBlock get last block
func (bc *BlockChain) GetHeadBlock() (*prototype.Block, error) {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	if bc.headBlock == nil {
		blk, err := bc.GetBlockByNum(bc.headBlockNum)
		if err != nil {
			return nil, err
		}

		if blk == nil {
			return nil, errors.Errorf("GetCurrentBlock: Not found block with number %d.", bc.headBlockNum)
		}

		bc.headBlock = blk
		return blk, nil
	} else {
		return bc.headBlock, nil
	}
}

// GetHeadBlockNum get number of last block in the chain
func (bc *BlockChain) GetHeadBlockNum() uint64 {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

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
	bc.lock.Lock()
	genesis := bc.genesisBlock
	bc.lock.Unlock()

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
