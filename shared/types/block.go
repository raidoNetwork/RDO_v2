package types

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/hasher"
	"time"
)

type BlockHeader struct {
	Num uint64
	Parent []byte
	Version []byte
	TxRoot	[]byte
}

type BlockSheet struct {
	Proposer *prototype.Sign
	Approvers []*prototype.Sign
	Slashers []*prototype.Sign
}

func NewBlock(header *BlockHeader, txBatch []*prototype.Transaction, blockSheet *BlockSheet) *prototype.Block {
	tstamp := uint64(time.Now().UnixNano())

	block := &prototype.Block{
		Num:          header.Num,
		Version:      header.Version,
		Hash:         hasher.BlockHash(header.Num, header.Version, header.Parent, header.TxRoot, tstamp),
		Parent:       header.Parent,
		Txroot:       header.TxRoot,
		Timestamp:    tstamp,
		Proposer:     blockSheet.Proposer,
		Transactions: txBatch,
	}

	return block
}

