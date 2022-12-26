package types

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

func NewOutput(address []byte, amount uint64, node []byte) *prototype.TxOutput {
	out := prototype.TxOutput{
		Address: address,
		Amount:  amount,
		Node:    node,
	}

	return &out
}

// NewInput Create new transaction input with output link to the transaction (hash and index)
func NewInput(hash []byte, index uint32, out *prototype.TxOutput) *prototype.TxInput {
	input := prototype.TxInput{
		Hash:    hash,
		Index:   index,
		Amount:  out.Amount,
		Address: out.Address,
		Node: out.Node,
	}

	return &input
}
