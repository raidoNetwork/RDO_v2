package types

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/utils/hash"
	"time"
)

var blockSigner BlockSigner
func GetBlockSigner() BlockSigner {
	// create block signer if not exist
	if blockSigner == nil {
		blockSigner = MakeBlockSigner("keccak256")
	}

	return blockSigner
}

type BlockHeader struct {
	Num uint64
	Parent []byte
	Version []byte
	TxRoot	[]byte
	Hash []byte
	Slot uint64
}

type BlockSheet struct {
	Proposer *prototype.Sign
	Approvers []*prototype.Sign
	Slashers []*prototype.Sign
}

func NewHeader(block *prototype.Block) *BlockHeader {
	return &BlockHeader{
		Num: block.Num,
		Parent: block.Parent,
		Version: block.Version,
		TxRoot: block.Txroot,
		Slot: block.Slot,
	}
}

func NewBlock(blockNum, slot uint64, parent []byte, txBatch []*prototype.Transaction, validator *keystore.ValidatorAccount) *prototype.Block {
	header := BlockHeader{
		Num:     blockNum,
		Parent:  parent,
		Version: []byte{1, 0, 0},
		TxRoot:  hash.GenTxRoot(txBatch),
		Slot: slot,
	}

	// sign block
	blockSheet := BlockSheet{
		Proposer: signBlock(&header, validator),
	}

	tstamp := uint64(time.Now().UnixNano())
	header.Hash = hash.BlockHash(header.Num, header.Slot, header.Version, header.Parent, header.TxRoot, tstamp)

	block := &prototype.Block{
		Num:          header.Num,
		Slot:         header.Slot,
		Version:      header.Version,
		Hash:         header.Hash,
		Parent:       header.Parent,
		Txroot:       header.TxRoot,
		Timestamp:    tstamp,
		Proposer:     blockSheet.Proposer,
		Transactions: txBatch,
	}

	// TODO add approvers and slashers

	return block
}

func signBlock(header *BlockHeader, validator *keystore.ValidatorAccount) *prototype.Sign {
	sign, err := GetBlockSigner().Sign(header, validator.Key())
	if err != nil {
		panic(errors.Wrap(err, "error signing block"))
	}

	return sign
}

