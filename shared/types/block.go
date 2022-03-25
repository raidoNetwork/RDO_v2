package types

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	hash2 "github.com/raidoNetwork/RDO_v2/utils/hash"
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
}

type BlockSheet struct {
	Proposer *prototype.Sign
	Approvers []*prototype.Sign
	Slashers []*prototype.Sign
}

func NewBlock(blockNum uint64, parent []byte, txBatch []*prototype.Transaction, validator *keystore.ValidatorAccount) *prototype.Block {
	header := BlockHeader{
		Num:     blockNum,
		Parent:  parent,
		Version: []byte{1, 0, 0},
		TxRoot:  hash2.GenTxRoot(txBatch),
	}

	// sign block
	blockSheet := BlockSheet{
		Proposer: signBlock(&header, validator),
	}

	tstamp := uint64(time.Now().UnixNano())
	header.Hash = hash2.BlockHash(header.Num, header.Version, header.Parent, header.TxRoot, tstamp)

	block := &prototype.Block{
		Num:          header.Num,
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

