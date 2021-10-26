package rdochain

import (
	"encoding/hex"
	ssz "github.com/ferranbt/fastssz"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"rdo_draft/blockchain/db"
	"rdo_draft/blockchain/db/kv"
	"rdo_draft/cmd/blockchain/flags"
	"rdo_draft/proto/prototype"
	"rdo_draft/shared/common"
	"rdo_draft/shared/crypto"
	"sync"
	"time"
)

const (
	txMinCout = 4 // minimum number of transactions in the block

	GenesisBlockNum = 0
)

var (
	GenesisHash = crypto.Keccak256Hash([]byte("genesis-hash"))
	signSuffix  = crypto.Keccak256Hash([]byte("something"))

	log = logrus.WithField("prefix", "rdochain")
)

// test node address
var nodeAddress = crypto.Keccak256Hash([]byte("super-node"))

func NewBlockChain(db db.BlockStorage, ctx *cli.Context) (*BlockChain, error) {
	bc := BlockChain{
		db:           db,
		prevHash:     GenesisHash,
		blockNum:     GenesisBlockNum + 1,             // first block has number genesis num + 1
		fullStatFlag: ctx.Bool(flags.LanSrvStat.Name), // stat flag

		lock: sync.RWMutex{},
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

	lock sync.RWMutex
}

// Init check database and update block num and previous hash
func (bc *BlockChain) Init() error {
	count, err := bc.db.CountBlocks()
	if err != nil {
		return err
	}

	// means that we need to add genesis
	if count == 0 {
		return nil
	}

	bc.blockNum = uint64(count) + 1 // future block num
	log.Infof("Database has %d blocks.", count)

	// get last block for previous hash
	block, err := bc.GetBlockByNum(count - 1)
	if err != nil {
		return err
	}

	if block == nil {
		log.Errorf("Not found block with num %d.", count)
		return errors.New("block not found")
	}

	bc.prevHash = block.Hash[:]

	return nil
}

// GenerateBlock creates block from given batch of transactions and store it to the database.
func (bc *BlockChain) GenerateBlock(tx []*prototype.Transaction) (*prototype.Block, error) {
	timestamp := time.Now().UnixNano()

	start := time.Now()

	// generate fee tx for block
	txFee, err := bc.createFeeTx(tx)
	if err != nil {
		return nil, err
	}

	tx = append(tx, txFee)

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("GenerateBlock: Create fee tx for address %s in %s.", hex.EncodeToString(nodeAddress), common.StatFmt(end))
	}

	start = time.Now()

	// generate reward tx for block
	txReward, err := bc.createRewardTx()
	if err != nil {
		return nil, err
	}

	tx = append(tx, txReward)

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("GenerateBlock: Create reward tx for address %s in %s.", hex.EncodeToString(nodeAddress), common.StatFmt(end))
	}

	start = time.Now()

	// create tx merklee tree root
	txRoot, err := bc.GenTxRoot(tx)
	if err != nil {
		return nil, err
	}

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("GenerateBlock: Create tx root in %s.", common.StatFmt(end))
	}

	// generate block hash
	h := bc.blockHash(bc.prevHash, bc.blockNum, txRoot, nodeAddress)

	block := &prototype.Block{
		Num:       bc.blockNum,
		Slot:      bc.blockNum,
		Version:   []byte{1, 0, 0},
		Hash:      h,
		Parent:    bc.prevHash,
		Txroot:    txRoot,
		Timestamp: uint64(timestamp),
		Size:      33,
		Proposer:  bc.sign(bc.prevHash, nodeAddress),
		Approvers: []*prototype.Sign{
			bc.sign(bc.prevHash, h),
			bc.sign(bc.prevHash, h),
			bc.sign(bc.prevHash, h),
		},
		Slashers: []*prototype.Sign{
			bc.sign(bc.prevHash, h),
			bc.sign(bc.prevHash, h),
		},
		Transactions: tx,
	}

	block.Size = uint32(block.SizeSSZ())

	bc.prevHash = h

	return block, nil
}

// SaveBlock stores given block in the database.
func (bc *BlockChain) SaveBlock(block *prototype.Block) error {
	bc.blockNum++

	return bc.db.WriteBlockWithNumKey(block)
}

// GetBlockByNum returns block from database by block number
func (bc *BlockChain) GetBlockByNum(num int) (*prototype.Block, error) {
	block, err := bc.db.ReadBlock(bc.genKey(num))
	if err != nil {
		return nil, err
	}

	return block, nil
}

// blockHash returns blockHash
func (bc *BlockChain) blockHash(prev []byte, num uint64, txroot []byte, proposer []byte) []byte {
	res := make([]byte, 0, 8)
	res = ssz.MarshalUint64(res, num)

	res = append(res, prev...)
	res = append(res, txroot...)
	res = append(res, proposer...)

	h := crypto.Keccak256Hash(res)
	res = h[:]

	return res
}

// sign test function for signing some data
func (bc *BlockChain) sign(prevHash []byte, hash []byte) *prototype.Sign {
	res := prevHash[:]

	if len(res) == 0 {
		res = crypto.Keccak256Hash([]byte("")) // null slice
	}

	res = append(res, hash...)
	res = append(res, signSuffix...) // need to fill sign to the size of 96 bytes

	sign := &prototype.Sign{
		Signature: res,
	}

	return sign
}

// genKey creates []byte key for database row by block num
func (bc *BlockChain) genKey(num int) []byte {
	nbyte := make([]byte, 0)
	nbyte = ssz.MarshalUint64(nbyte, uint64(num))
	nbyte = append(nbyte, kv.BlockSuffix...)
	key := crypto.Keccak256Hash(nbyte)

	return key
}

// GetBlockCount return current block number
func (bc *BlockChain) GetBlockCount() uint64 {
	bc.lock.RLock()
	num := bc.blockNum
	bc.lock.RUnlock()

	return num
}

// GenTxRoot create transactions root hash of block
func (bc *BlockChain) GenTxRoot(txarr []*prototype.Transaction) ([]byte, error) {
	data := make([][]byte, 0, len(txarr))

	for _, tx := range txarr {
		data = append(data, tx.Hash)
	}

	root, err := merkleeRoot(data)
	if err != nil {
		return nil, err
	}

	return root, nil
}

// merkleeRoot return root hash with given []byte
func merkleeRoot(data [][]byte) (res []byte, err error) {
	size := len(data)

	lvlKoef := size % 2
	lvlCount := size/2 + lvlKoef
	lvlSize := size

	prevLvl := data
	tree := make([][][]byte, lvlCount)

	var mix []byte
	for i := 0; i < lvlCount; i++ {
		tree[i] = make([][]byte, 0)

		for j := 0; j < lvlSize; j += 2 {
			next := j + 1

			if next == lvlSize {
				mix = prevLvl[j]
			} else {
				mix = prevLvl[j]
				mix = append(mix, prevLvl[next]...)
			}

			mix = crypto.Keccak256Hash(mix)

			tree[i] = append(tree[i], mix)
		}

		lvlSize = len(tree[i])
		prevLvl = tree[i]
	}

	d := tree[len(tree)-1]

	if len(d) > 1 {
		return nil, errors.New("Error creating MerkleeTree")
	}

	res = d[0]

	return res, nil
}
