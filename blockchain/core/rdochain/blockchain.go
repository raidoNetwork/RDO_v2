package rdochain

import (
	"bytes"
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
	"strconv"
	"sync"
	"time"
)

const (
	txMinCout = 4 // minimum number of transactions in the block

	slotTime = 1 * time.Second
)

var (
	GenesisHash = crypto.Keccak256Hash([]byte("genesis-hash"))
	signSuffix  = crypto.Keccak256Hash([]byte("something"))

	log = logrus.WithField("prefix", "rdochain")
)

var nodeAddress = crypto.Keccak256Hash([]byte("super-node"))

func NewBlockChain(db db.BlockStorage, ctx *cli.Context) (*BlockChain, error) {
	bc := BlockChain{
		db:           db,
		prevHash:     GenesisHash,
		blockNum:     1,
		fullStatFlag: ctx.Bool(flags.LanSrvStat.Name),

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

	bc.blockNum = uint64(count) + 1 // current block num
	log.Infof("Database has %d blocks.", count)

	// get last block for previous hash
	block, err := bc.GetBlockByNum(count)
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

	err = bc.ValidateBlock(block)
	if err != nil {
		return nil, err
	}

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("Validate block in %s", common.StatFmt(end))
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

// ValidateBlock validates given block.
func (bc *BlockChain) ValidateBlock(block *prototype.Block) error {
	// check that block has no double in outputs and inputs
	inputExists := map[string]string{}
	outputExists := map[string]string{}

	start := time.Now()

	var blockBalance uint64 = 0

	for txIndex, tx := range block.Transactions {
		// skip reward tx
		if tx.Type == common.RewardTxType {
			if len(tx.Outputs) != 1 {
				return errors.New("Wrong tx reward outputs size.")
			}

			if tx.Outputs[0].Amount != bc.GetReward() {
				return errors.New("Wrong block reward given.")
			}

			continue
		}

		// check inputs
		for _, in := range tx.Inputs {
			key := hex.EncodeToString(in.Hash) + "_" + strconv.Itoa(int(in.Index))

			hash, exists := inputExists[key]
			if exists {
				curHash := hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(txIndex)
				return errors.Errorf("Block #%d has double input in tx %s with key %s. Bad tx: %s.", block.Num, hash, key, curHash)
			}

			inputExists[key] = hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(txIndex)
			blockBalance += in.Amount
		}

		// check outputs
		for outIndex, out := range tx.Outputs {
			key := hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(outIndex)

			hash, exists := outputExists[key]
			if exists {
				curHash := hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(txIndex)
				return errors.Errorf("Block #%d has double output in tx %s with key %s. Bad tx: %s.", block.Num, hash, key, curHash)
			}

			outputExists[key] = hex.EncodeToString(tx.Hash) + "_" + strconv.Itoa(txIndex)
			blockBalance -= out.Amount
		}
	}

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("Check block tx doubles in %s", common.StatFmt(end))
	}

	if blockBalance != 0 {
		return errors.New("Wrong block balance.")
	}

	// check block tx root
	txRoot, err := bc.GenTxRoot(block.Transactions)
	if err != nil {
		log.Error("BlockChain.ValidateBlock: error creating tx root.")
		return err
	}

	if !bytes.Equal(txRoot, block.Txroot) {
		return errors.Errorf("Block tx root mismatch. Given: %s. Expected: %s.", hex.EncodeToString(block.Txroot), hex.EncodeToString(txRoot))
	}

	tstamp := time.Now().UnixNano() + int64(common.SlotTime)
	if tstamp < int64(block.Timestamp) {
		return errors.Errorf("Wrong block timestamp: %d. Timestamp with slot time: %d.", block.Timestamp, tstamp)
	}

	start = time.Now()

	// check if block exists
	b, err := bc.GetBlockByNum(int(block.Num))
	if err != nil {
		return errors.New("Error reading block from database.")
	}

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("Get block by num in %s", common.StatFmt(end))
	}

	if b != nil {
		return errors.Errorf("Block #%d is already exists in blockchain!", block.Num)
	}

	start = time.Now()

	// find prevBlock
	prevBlockNum := int(block.Num) - 1
	prevBlock, err := bc.GetBlockByNum(prevBlockNum)
	if err != nil {
		return errors.New("Error reading block from database.")
	}

	if bc.fullStatFlag {
		end := time.Since(start)
		log.Infof("Check prev block in %s", common.StatFmt(end))
	}

	if prevBlock == nil {
		// check if prevBlock is genesis
		if prevBlockNum == 0 {
			// add genesis check
		} else {
			return errors.Errorf("Previous Block #%d for given block #%d is not exists.", block.Num-1, block.Num)
		}
	}

	if prevBlock.Timestamp >= block.Timestamp {
		return errors.Errorf("Timestamp is too small. Previous: %d. Current: %d.", prevBlock.Timestamp, block.Timestamp)
	}

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
