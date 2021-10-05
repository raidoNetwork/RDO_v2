package rdochain

import (
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
	"time"
)

const (
	txMinCout = 4 // minimum number of transactions in the block
)

var (
	GenesisHash = crypto.Keccak256Hash([]byte("genesis-hash"))
	signSuffix  = crypto.Keccak256Hash([]byte("something"))

	log = logrus.WithField("prefix", "rdochain")
)

func NewBlockChain(db db.BlockStorage, ctx *cli.Context) (*BlockChain, error) {
	bc := BlockChain{
		db:           db,
		prevHash:     GenesisHash,
		blockNum:     1,
		fullStatFlag: ctx.Bool(flags.LanSrvStat.Name),
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
}

// Init check database and update block num and previous hash
func (bc *BlockChain) Init() error {
	count, err := bc.db.CountBlocks()
	if err != nil {
		return err
	}

	// means that
	if count == 0 {
		return nil
	}

	bc.blockNum = uint64(count) + 1
	log.Infof("Database has %d blocks.", count)

	// get last block for previous hash
	block, err := bc.GetBlockByNum(count)
	if err != nil {
		return err
	}

	if block == nil {
		log.Info("Block not found error")
		return errors.New("block not found")
	}

	bc.prevHash = block.Hash[:]

	return nil
}

// GenerateAndSaveBlock test function for creating and storing block from given batch of tx.
func (bc *BlockChain) GenerateAndSaveBlock(tx []*prototype.Transaction) (*prototype.Block, error) {
	start := time.Now()

	block, err := bc.GenerateBlock(tx)
	if err != nil {
		return nil, err
	}

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("Generate block in %s", common.StatFmt(end))
	}

	start = time.Now()

	err = bc.SaveBlock(block)
	if err != nil {
		return nil, err
	}

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("Store block in %s", common.StatFmt(end))
	}

	return block, nil
}

// GenerateBlock creates block from given batch of transactions and store it to the database.
func (bc *BlockChain) GenerateBlock(tx []*prototype.Transaction) (*prototype.Block, error) {
	timestamp := time.Now().UnixNano()
	h := bc.blockHash(bc.prevHash, bc.blockNum, timestamp) // hash

	block := &prototype.Block{
		Num:       bc.blockNum,
		Slot:      bc.blockNum,
		Version:   []byte{1, 0, 0},
		Hash:      h,
		Parent:    bc.prevHash,
		Txroot:    h,
		Timestamp: uint64(timestamp),
		Size:      33,
		Proposer:  bc.sign(bc.prevHash, h),
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

	bc.blockNum++
	bc.prevHash = h

	return block, nil
}

// SaveBlock stores given block in the database.
func (bc *BlockChain) SaveBlock(block *prototype.Block) error {
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

func (bc *BlockChain) hashDomain(num uint64, timestamp int64) []byte {
	data := make([]byte, 0, 16)
	data = ssz.MarshalUint64(data, num)
	data = ssz.MarshalUint64(data, uint64(timestamp))

	return data
}

func (bc *BlockChain) blockHash(prev []byte, num uint64, timestamp int64) []byte {
	res := bc.hashDomain(num, timestamp)
	res = append(res, prev...)

	h := crypto.Keccak256Hash(res)
	res = h[:]

	return res
}

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

func (bc *BlockChain) genKey(num int) []byte {
	nbyte := make([]byte, 0)
	nbyte = ssz.MarshalUint64(nbyte, uint64(num))
	nbyte = append(nbyte, kv.BlockSuffix...)
	key := crypto.Keccak256Hash(nbyte)

	return key
}

func (bc *BlockChain) GetBlockCount() uint64 {
	return bc.blockNum
}
