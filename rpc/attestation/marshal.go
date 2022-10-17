package attestation

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/v2/utilities"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/rpc/cast"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"io"
	"strings"
)

// MarshalTx convert JSON struct to the
func MarshalTx(data string) string {
	return common.Encode([]byte(data))
}

func UnmarshalTx(enc string) (*prototype.Transaction, error) {
	data := string(common.FromHex(enc))

	var protoReq prototype.SignedTxValue
	marshaler := runtime.JSONPb{}

	newReader, berr := utilities.IOReaderFactory(strings.NewReader(data))
	if berr != nil {
		return nil, berr
	}
	if err := marshaler.NewDecoder(newReader()).Decode(&protoReq); err != nil && err != io.EOF {
		return nil, err
	}

	tx := cast.TxFromTxValue(&protoReq)
	if tx == nil {
		return nil, errors.New("Empty transaction data")
	}

	return tx, nil
}
