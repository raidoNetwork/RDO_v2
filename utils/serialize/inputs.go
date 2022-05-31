package serialize

import (
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"strconv"
)

func GenKeyFromInput(in *prototype.TxInput) string {
	return common.Encode(in.Hash) + "_" + strconv.Itoa(int(in.Index))
}
