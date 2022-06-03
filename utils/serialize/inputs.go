package serialize

import (
	"fmt"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"strconv"
)

func GenKeyFromPbInput(in *prototype.TxInput) string {
	return common.Encode(in.Hash) + "_" + strconv.Itoa(int(in.Index))
}

func GenKeyFromInput(in *types.Input) string {
	return fmt.Sprintf("%s_%d", in.Hash().Hex(), in.Index())
}