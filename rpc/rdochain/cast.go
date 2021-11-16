package rdochain

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
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
