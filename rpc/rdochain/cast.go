package rdochain

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
)

func convertProtoToInner(uo *types.UTxO) *prototype.UTxO {
	uopb := new(prototype.UTxO)

	uopb.BlockNum = uo.BlockNum
	uopb.Hash = uo.Hash.Bytes()
	uopb.Index = uo.Index
	uopb.From = uo.From.Bytes()
	uopb.To = uo.To.Bytes()
	uopb.Node = uo.Node.Bytes()
	uopb.Amount = uo.Amount
	uopb.Timestamp = uo.Timestamp
	uopb.Txtype = int32(uo.TxType)

	return uopb
}

func convBlock(block *prototype.Block) *prototype.BlockValue {
	bv := new(prototype.BlockValue)

	bv.Num = block.Num
	bv.Hash = convHash(block.Hash)
	bv.Parent = convHash(block.Parent)
	bv.Timestamp = block.Timestamp
	bv.Proposer = convSign(block.Proposer)

	size := len(block.Approvers)
	bv.Approvers = make([]string, size)

	for i := 0; i < size; i++ {
		bv.Approvers[i] = convSign(block.Approvers[i])
	}

	size = len(block.Slashers)
	bv.Slashers = make([]string, size)

	for i := 0; i < size; i++ {
		bv.Slashers[i] = convSign(block.Slashers[i])
	}

	size = len(block.Transactions)
	bv.Transactions = make([]*prototype.TxValue, size)

	for i := 0; i < size; i++ {
		bv.Transactions[i] = convTx(block.Transactions[i])
	}

	return bv
}

func convSign(s *prototype.Sign) string {
	return convHash(s.Address)
}

func convAddress(a []byte) string {
	return common.BytesToAddress(a).Hex()
}

func convHash(h []byte) string {
	return common.BytesToHash(h).Hex()
}

func convTx(tx *prototype.Transaction) *prototype.TxValue {
	tv := new(prototype.TxValue)

	tv.Num = tx.Num
	tv.Type = tx.Type
	tv.Timestamp = tx.Timestamp
	tv.Hash = convHash(tx.Hash)
	tv.Fee = tx.Fee
	tv.Data = tx.Data

	size := len(tx.Inputs)
	tv.Inputs = make([]*prototype.TxInputValue, size)

	for i := 0; i < size; i++ {
		tv.Inputs[i] = convInput(tx.Inputs[i])
	}

	size = len(tx.Outputs)
	tv.Outputs = make([]*prototype.TxOutputValue, size)

	for i := 0; i < size; i++ {
		tv.Outputs[i] = convOutput(tx.Outputs[i])
	}

	return tv
}

func convInput(in *prototype.TxInput) *prototype.TxInputValue {
	inv := new(prototype.TxInputValue)

	inv.Hash = convHash(in.Hash)
	inv.Index = in.Index
	inv.Address = convAddress(in.Address)
	inv.Amount = in.Amount

	return inv
}

func convOutput(out *prototype.TxOutput) *prototype.TxOutputValue {
	outv := new(prototype.TxOutputValue)

	outv.Address = convAddress(out.Address)
	outv.Amount = out.Amount
	outv.Node = convHash(out.Node)

	return outv
}
