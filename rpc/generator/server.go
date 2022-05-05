package generator

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/rpc/api"
	"github.com/raidoNetwork/RDO_v2/rpc/cast"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"google.golang.org/grpc"
)

type Server struct {
	Server  *grpc.Server
	Backend api.GeneratorAPI

	prototype.UnimplementedGeneratorServer
}

func (s *Server) CreateTx(ctx context.Context, req *prototype.TxOptionsRequest) (*prototype.TxBodyResponse, error) {
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	if len(req.Outputs) == 0 {
		return nil, errors.New("Wrong outputs count.")
	}

	if !common.IsHexHash(req.GetKey()) {
		return nil, errors.New("Wrong private key given.")
	}

	arr := make([]*prototype.TxOutput, len(req.Outputs))
	for i, outv := range req.Outputs {
		arr[i] = cast.TxOutput(outv)
	}

	tx, err := s.Backend.GenerateTx(arr, req.Fee, req.Key)
	if err != nil {
		return nil, err
	}

	res := s.createTxBodyResponse(tx)

	return res, nil
}

func (s *Server) CreateStakeTx(ctx context.Context, req *prototype.TxOptionsStakeRequest) (*prototype.TxBodyResponse, error) {
	return s.createTxStruct("stake", req)
}

func (s *Server) CreateUnstakeTx(ctx context.Context, req *prototype.TxOptionsStakeRequest) (*prototype.TxBodyResponse, error) {
	return s.createTxStruct("unstake", req)
}

func (s *Server) createTxStruct(method string, req *prototype.TxOptionsStakeRequest) (*prototype.TxBodyResponse, error) {
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	if !common.IsHexHash(req.GetKey()) {
		return nil, errors.New("Wrong private key given.")
	}

	var tx *prototype.Transaction

	if method == "stake" {
		tx, err = s.Backend.GenerateStakeTx(req.Fee, req.Key, req.Amount)
	} else {
		tx, err = s.Backend.GenerateUnstakeTx(req.Fee, req.Key, req.Amount)
	}

	if err != nil {
		return nil, err
	}

	res := s.createTxBodyResponse(tx)

	return res, nil
}

func (s *Server) createTxBodyResponse(tx *prototype.Transaction) *prototype.TxBodyResponse {
	res := new(prototype.TxBodyResponse)
	res.Tx = cast.SignedTxValue(tx)

	return res
}
