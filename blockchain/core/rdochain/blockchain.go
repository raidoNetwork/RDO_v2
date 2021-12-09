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

var (
	GenesisHash = crypto.Keccak256([]byte("genesis-hash"))

	log = logrus.WithField("prefix", "rdochain")
)

// nodeAddress hex - a91b3079bdbf477fb8bf39eb329052e286ae47d5e878f5f5d9e70737296cb25b
var nodeAddress = crypto.Keccak256Hash([]byte("super-node"))

func NewBlockChain(db db.BlockStorage, ctx *cli.Context, cfg *params.RDOBlockChainConfig) (*BlockChain, error) {
	bc := BlockChain{
		db:              db,
		prevHash:        GenesisHash,
		currentBlockNum: GenesisBlockNum,
		blockNum:        GenesisBlockNum + 1,          // block num for the future block
		fullStatFlag:    ctx.Bool(flags.SrvStat.Name), // stat flag

		lock: sync.RWMutex{},
		cfg: cfg,
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
	blockNum     uint64
	fullStatFlag bool

	currentBlockNum uint64
	currentBlock    *prototype.Block
	genesisBlock    *prototype.Block

	lock sync.RWMutex

	cfg *params.RDOBlockChainConfig
}

// Init check database and update block num and previous hash
func (bc *BlockChain) Init() error {
	log.Info("Init blockchain data.")

	// insert Genesis if not exists
	err := bc.insertGenesis()
	if err != nil {
		return err
	}

	// get head block
	head, err := bc.db.GetHeadBlockNum()
	if err != nil {
		if errors.Is(err, kv.ErrNoHead) {
			start := time.Now()

			count, err := bc.db.CountBlocks()
			if err != nil {
				return err
			}

			if bc.fullStatFlag {
				end := time.Since(start)
				log.Infof("Init: Count KV rows num in %s.", common.StatFmt(end))
			}

			// save head block link
			head = uint64(count) - 1 // skip genesis key
			err = bc.SaveHeadBlockNum(head)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	var block *prototype.Block

	// update counter according to the database
	bc.currentBlockNum = head            // head block number
	bc.blockNum = bc.currentBlockNum + 1 // future block num

	if head != 0 {
		// get last block
		block, err = bc.GetBlockByNum(bc.currentBlockNum)
	} else {
		// if head equal to zero return Genesis
		block, err = bc.db.GetGenesis()
	}

	if err != nil {
		return err
	}

	if block == nil {
		log.Errorf("Init: Not found block with num %d.", bc.currentBlockNum)
		return errors.New("block not found")
	}

	log.Infof("Database has %d blocks. Future block num %d.", bc.currentBlockNum, bc.blockNum)

	bc.prevHash = block.Hash[:]
	bc.currentBlock = block

	return nil
}

// GenerateBlock creates block from given batch of transactions and store it to the database.
func (bc *BlockChain) GenerateBlock(tx []*prototype.Transaction) (*prototype.Block, error) {
	timestamp := time.Now().UnixNano()

	// generate fee tx for block
	if len(tx) > 0 {
		txFee, err := bc.createFeeTx(tx)
		if err != nil {
			if !errors.Is(err, ErrZeroFeeAmount) {
				return nil, err
			}
		} else {
			tx = append(tx, txFee)
		}
	}

	start := time.Now()

	// create tx merklee tree root
	txRoot, err := bc.GenTxRoot(tx)
	if err != nil {
		return nil, err
	}

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("GenerateBlock: Create tx root in %s.", common.StatFmt(end))
	}

	// get num for future block
	blockNum := bc.GetBlockCount()

	// generate block hash
	hash := hasher.BlockHash(blockNum, bc.prevHash, txRoot, nodeAddress.Bytes())

	block := &prototype.Block{
		Num:       blockNum,
		Version:   []byte{1, 0, 0},
		Hash:      hash,
		Parent:    bc.prevHash,
		Txroot:    txRoot,
		Timestamp: uint64(timestamp),
		Proposer:  bc.sign(nodeAddress.Bytes()),
		Approvers: []*prototype.Sign{
			bc.sign(hash),
			bc.sign(hash),
			bc.sign(hash),
		},
		Slashers: []*prototype.Sign{
			bc.sign(hash),
			bc.sign(hash),
		},
		Transactions: tx,
	}

	return block, nil
}

func (bc *BlockChain) SaveHeadBlockNum(n uint64) error {
	return bc.db.SaveHeadBlockNum(n)
}

// SaveBlock stores given block in the database.
func (bc *BlockChain) SaveBlock(block *prototype.Block) error {
	err := bc.db.WriteBlock(block)
	if err != nil {
		return err
	}

	err = bc.SaveHeadBlockNum(block.Num)
	if err != nil {
		return err
	}

	// update blocks stats
	bc.lock.Lock()
	bc.currentBlockNum++
	bc.blockNum++
	bc.prevHash = block.Hash
	bc.currentBlock = block
	bc.lock.Unlock()

	log.Warn("Block data was successfully saved.")

	return nil
}

// GetBlockByNum returns block from database by block number
func (bc *BlockChain) GetBlockByNum(num uint64) (*prototype.Block, error) {
	if num == 0 {
		return bc.genesisBlock, nil
	}

	return bc.db.GetBlockByNum(num)
}

// GetBlockByHash returns block
func (bc *BlockChain) GetBlockByHash(hash []byte) (*prototype.Block, error) {
	if bytes.Equal(hash, GenesisHash) {
		return bc.genesisBlock, nil
	}

	return bc.db.GetBlockByHash(hash)
}

// sign test function for signing some data
func (bc *BlockChain) sign(addr []byte) *prototype.Sign {
	randSign := make([]byte, crypto.SignatureLength)
	rand.Read(randSign)

	sign := &prototype.Sign{
		Address:   addr,
		Signature: randSign,
	}

	return sign
}

// GetBlockCount return block count
func (bc *BlockChain) GetBlockCount() uint64 {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	return bc.blockNum
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

// GetCurrentBlock get last block
func (bc *BlockChain) GetCurrentBlock() (*prototype.Block, error) {
	if bc.currentBlock == nil {
		blk, err := bc.GetBlockByNum(bc.currentBlockNum)
		if err != nil {
			return nil, err
		}

		if blk == nil {
			return nil, errors.Errorf("GetCurrentBlock: Not found block with number %d.", bc.currentBlockNum)
		}

		bc.currentBlock = blk
		return blk, nil
	} else {
		return bc.currentBlock, nil
	}
}

// GetCurrentBlockNum get number of last block in the chain
func (bc *BlockChain) GetCurrentBlockNum() uint64 {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	return bc.currentBlockNum
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