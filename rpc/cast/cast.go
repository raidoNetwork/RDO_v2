package cast

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "cast")

func ProtoUTxO(uo *types.UTxO) *prototype.UTxO {
	uopb := new(prototype.UTxO)

	uopb.BlockNum = uo.BlockNum
	uopb.Hash = uo.Hash.Hex()
	uopb.Index = uo.Index
	uopb.From = uo.From.Hex()
	uopb.To = uo.To.Hex()
	uopb.Node = uo.Node.Hex()
	uopb.Amount = uo.Amount
	uopb.Timestamp = uo.Timestamp
	uopb.Txtype = uo.TxType

	return uopb
}

func BlockValue(block *prototype.Block) *prototype.BlockValue {
	bv := new(prototype.BlockValue)

	bv.Num = block.Num
	bv.Slot = block.Slot
	bv.Hash = HashString(block.Hash)
	bv.Parent = HashString(block.Parent)
	bv.Timestamp = block.Timestamp
	bv.Proposer = ConvSign(block.Proposer)

	size := len(block.Approvers)
	bv.Approvers = make([]string, size)

	for i := 0; i < size; i++ {
		bv.Approvers[i] = ConvSign(block.Approvers[i])
	}

	size = len(block.Slashers)
	bv.Slashers = make([]string, size)

	for i := 0; i < size; i++ {
		bv.Slashers[i] = ConvSign(block.Slashers[i])
	}

	size = len(block.Transactions)
	bv.Transactions = make([]*prototype.TxValue, size)

	for i := 0; i < size; i++ {
		bv.Transactions[i] = TxValue(block.Transactions[i])
	}

	return bv
}

func ConvSign(s *prototype.Sign) string {
	return AddressString(s.Address)
}

func AddressString(a []byte) string {
	return common.BytesToAddress(a).Hex()
}

func HashString(h []byte) string {
	return common.BytesToHash(h).Hex()
}

func TxValue(tx *prototype.Transaction) *prototype.TxValue {
	tv := new(prototype.TxValue)

	tv.Num = tx.Num
	tv.Type = tx.Type
	tv.Timestamp = tx.Timestamp
	tv.Hash = HashString(tx.Hash)
	tv.Fee = tx.Fee
	tv.Data = tx.Data

	size := len(tx.Inputs)
	tv.Inputs = make([]*prototype.TxInputValue, size)

	for i := 0; i < size; i++ {
		tv.Inputs[i] = TxInputValue(tx.Inputs[i])
	}

	size = len(tx.Outputs)
	tv.Outputs = make([]*prototype.TxOutputValue, size)

	for i := 0; i < size; i++ {
		tv.Outputs[i] = TxOutputValue(tx.Outputs[i])
	}

	return tv
}

func TxInputValue(in *prototype.TxInput) *prototype.TxInputValue {
	inv := new(prototype.TxInputValue)

	inv.Hash = HashString(in.Hash)
	inv.Index = in.Index
	inv.Address = AddressString(in.Address)
	inv.Amount = in.Amount
	inv.Node = AddressString(in.Node)

	return inv
}

func TxOutputValue(out *prototype.TxOutput) *prototype.TxOutputValue {
	outv := new(prototype.TxOutputValue)

	outv.Address = AddressString(out.Address)
	outv.Amount = out.Amount
	outv.Node = AddressString(out.Node)

	return outv
}

func TxOutput(outv *prototype.TxOutputValue) *prototype.TxOutput {
	out := types.NewOutput(
		common.HexToAddress(outv.Address).Bytes(),
		outv.Amount,
		common.HexToAddress(outv.Node).Bytes(),
	)

	return out
}

func TxInput(inv *prototype.TxInputValue) *prototype.TxInput {
	in := new(prototype.TxInput)

	in.Index = inv.Index
	in.Amount = inv.Amount
	in.Hash = common.HexToHash(inv.Hash).Bytes()
	in.Address = common.HexToAddress(inv.Address).Bytes()
	in.Node = common.HexToAddress(inv.Node).Bytes()

	return in
}

func SignedTxValue(tx *prototype.Transaction) *prototype.SignedTxValue {
	txv := new(prototype.SignedTxValue)
	txv.Data = TxValue(tx)
	txv.Signature = common.Encode(tx.Signature)
	return txv
}

func NotSignedTxValue(tx *prototype.Transaction) *prototype.NotSignedTxValue {
	var txv prototype.NotSignedTxValue
	txv.Data = TxValue(tx)
	txv.Signature = "sign_here"

	return &txv
}

func TxFromTxValue(txv *prototype.SignedTxValue) *prototype.Transaction {
	if txv == nil || txv.Data == nil {
		return nil
	}

	tx := new(prototype.Transaction)

	tx.Num = txv.Data.Num
	tx.Type = txv.Data.Type
	tx.Fee = txv.Data.Fee
	tx.Timestamp = txv.Data.Timestamp
	tx.Data = txv.Data.Data
	tx.Hash = common.HexToHash(txv.Data.Hash).Bytes()

	tx.Inputs = make([]*prototype.TxInput, len(txv.Data.Inputs))
	for i, inv := range txv.Data.Inputs {
		tx.Inputs[i] = TxInput(inv)
	}

	tx.Outputs = make([]*prototype.TxOutput, len(txv.Data.Outputs))
	for i, out := range txv.Data.Outputs {
		tx.Outputs[i] = TxOutput(out)
	}

	tx.Signature = common.FromHex(txv.Signature)

	return tx
}
