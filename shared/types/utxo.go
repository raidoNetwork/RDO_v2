package types

import (
	"fmt"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"time"
)

type UTxO struct {
	BlockNum  uint64         `json:"blockNum"`
	Hash      common.Hash    `json:"hash"`
	Index     uint32         `json:"index"`
	From      common.Address `json:"from"`
	To        common.Address `json:"to"`
	Node      common.Hash    `json:"node"`
	Amount    uint64         `json:"amount"`
	Timestamp uint64         `json:"timestamp"`
	ID     uint64
	Spent  uint64
	TxType int    			 `json:"txType"`
}

func (uo *UTxO) ToOutput() *prototype.TxOutput {
	return NewOutput(uo.To.Bytes(), uo.Amount, uo.Node.Bytes())
}

func (uo *UTxO) ToInput() *prototype.TxInput {
	in, err := NewInput(uo.Hash.Bytes(), uo.Index, uo.ToOutput(), nil)
	if err != nil {
		return nil
	}

	return in
}

func (uo *UTxO) ToString() string {
	return fmt.Sprintf("ID: %d Type: %d Hash: %s From: %s To: %s Node: %s Amount: %d  Spent: %d BlockNum: %d Timestamp %d",
		uo.ID,
		uo.TxType,
		uo.Hash,
		uo.From,
		uo.To,
		uo.Node,
		uo.Amount,
		uo.Spent,
		uo.BlockNum,
		uo.Timestamp)
}

func NewUTxO(hash, from, to, node []byte, index uint32, amount uint64, blockNum uint64, typev int, tstamp uint64) *UTxO {
	if tstamp == 0 {
		tstamp = uint64(time.Now().UnixNano())
	}

	uo := UTxO{
		Hash:      common.BytesToHash(hash),
		Index:     index,
		From:      common.BytesToAddress(from),
		To:        common.BytesToAddress(to),
		Node:      common.BytesToHash(node),
		Amount:    amount,
		Timestamp: tstamp,
		BlockNum:  blockNum,
		TxType:    typev,
	}

	return &uo
}

func NewUTxOFull(id uint64, hash, from, to, node string, index uint32, amount, blockNum, unspent, timestamp uint64, typev int) (*UTxO, error) {
	uo := UTxO{
		ID:        id,
		Hash:      common.HexToHash(hash),
		From:      common.HexToAddress(from),
		To:        common.HexToAddress(to),
		Node:      common.HexToHash(node),
		Index:     index,
		Amount:    amount,
		BlockNum:  blockNum,
		Timestamp: timestamp,
		Spent:     unspent,
		TxType:    typev,
	}

	return &uo, nil
}
