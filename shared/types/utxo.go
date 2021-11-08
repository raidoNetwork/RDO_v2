package types

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"time"
)

type UTxO struct {
	BlockNum  uint64
	Hash      []byte
	Index     uint32
	From      []byte
	To        []byte
	Amount    uint64
	Timestamp uint64

	ID     uint64
	Spent  uint64
	TxType int

	idIndex string
	node    []byte
}

func (uo *UTxO) ToOutput() *prototype.TxOutput {
	return NewOutput(uo.To, uo.Amount)
}

func (uo *UTxO) ToInput(key *ecdsa.PrivateKey) *prototype.TxInput {
	in, err := NewInput(uo.Hash, uo.Index, uo.ToOutput(), key)
	if err != nil {
		return nil
	}

	return in
}

func (uo *UTxO) ToString() string {
	hash := hex.EncodeToString(uo.Hash)
	from := hex.EncodeToString(uo.From)
	to := hex.EncodeToString(uo.To)

	return fmt.Sprintf("ID: %d Type: %d Hash: %s From: %s To: %s Amount: %d  Spent: %d BlockNum: %d Timestamp %d",
		uo.ID,
		uo.TxType,
		hash,
		from,
		to,
		uo.Amount,
		uo.Spent,
		uo.BlockNum,
		uo.Timestamp)
}

func NewUTxO(hash, from, to []byte, index uint32, amount uint64, blockNum uint64, typev int) *UTxO {
	uo := UTxO{
		Hash:      hash,
		Index:     index,
		From:      from,
		To:        to,
		Amount:    amount,
		Timestamp: uint64(time.Now().UnixNano()),
		BlockNum:  blockNum,
		TxType:    typev,
	}

	return &uo
}

func NewUTxOFull(id uint64, hash, from, to string, index uint32, amount, blockNum, unspent, timestamp uint64, typev int) (*UTxO, error) {
	bhash, err := hex.DecodeString(hash)
	if err != nil {
		return nil, err
	}

	bfrom, err := hex.DecodeString(from)
	if err != nil {
		return nil, err
	}

	bto, err := hex.DecodeString(to)
	if err != nil {
		return nil, err
	}

	uo := UTxO{
		ID:        id,
		Hash:      bhash,
		From:      bfrom,
		To:        bto,
		Index:     index,
		Amount:    amount,
		BlockNum:  blockNum,
		Timestamp: timestamp,
		Spent:     unspent,
		TxType:    typev,
	}

	return &uo, nil
}
