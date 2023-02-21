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
	Server      *grpc.Server
	Backend     api.GeneratorAPI
	Attestation api.AttestationAPI

	prototype.UnimplementedGeneratorServer
}

func (s *Server) UnsafeSend(ctx context.Context, req *prototype.TxOptionsUnsafeRequest) (*prototype.TxBodyUnsafeResponse, error) {
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

	tx, err := s.Backend.GenerateUnsafeTx(arr, req.Fee, req.Key)
	if err != nil {
		return nil, err
	}

	res := s.createTxUnsafeBodyResponse(tx)

	return res, nil
}

func (s *Server) UnsafeStakeTx(ctx context.Context, req *prototype.TxOptionsStakeUnsafeRequest) (*prototype.TxBodyUnsafeResponse, error) {
	return s.createTxUnsafeStruct("stake", req)
}

func (s *Server) UnsafeUnstakeTx(ctx context.Context, req *prototype.TxOptionsStakeUnsafeRequest) (*prototype.TxBodyUnsafeResponse, error) {
	return s.createTxUnsafeStruct("unstake", req)
}

func (s *Server) createTxUnsafeStruct(method string, req *prototype.TxOptionsStakeUnsafeRequest) (*prototype.TxBodyUnsafeResponse, error) {
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	if !common.IsHexHash(req.GetKey()) {
		return nil, errors.New("Wrong private key given.")
	}

	var tx *prototype.Transaction

	if method == "stake" {
		tx, err = s.Backend.GenerateUnsafeStakeTx(req.Fee, req.Key, req.Amount, req.Node)
	} else {
		tx, err = s.Backend.GenerateUnsafeUnstakeTx(req.Fee, req.Key, req.Amount, req.Node)
	}

	if err != nil {
		return nil, err
	}

	res := s.createTxUnsafeBodyResponse(tx)

	return res, nil
}

func (s *Server) createTxUnsafeBodyResponse(tx *prototype.Transaction) *prototype.TxBodyUnsafeResponse {
	res := new(prototype.TxBodyUnsafeResponse)
	res.Tx = cast.SignedTxValue(tx)

	return res
}

func (s *Server) Send(ctx context.Context, req *prototype.TxOptionsRequest) (*prototype.TxBodyResponse, error) {
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	if len(req.Outputs) == 0 {
		return nil, errors.New("Wrong outputs count.")
	}

	// Validate address
	if !common.IsHexAddress(req.GetAddress()) {
		return nil, errors.New("Wrong address given.")
	}

	arr := make([]*prototype.TxOutput, len(req.Outputs))
	for i, outv := range req.Outputs {
		arr[i] = cast.TxOutput(outv)
	}

	tx, err := s.Backend.GenerateTx(arr, req.Fee, req.Address)
	if err != nil {
		return nil, err
	}

	res := s.createTxBodyResponse(tx)

	return res, nil
}

func (s *Server) StakeTx(ctx context.Context, req *prototype.TxOptionsStakeRequest) (*prototype.TxBodyResponse, error) {
	return s.createTxStruct("stake", req)

}

func (s *Server) UnstakeTx(ctx context.Context, req *prototype.TxOptionsStakeRequest) (*prototype.TxBodyResponse, error) {
	return s.createTxStruct("unstake", req)

}

func (s *Server) createTxStruct(method string, req *prototype.TxOptionsStakeRequest) (*prototype.TxBodyResponse, error) {
	err := req.Validate()
	if err != nil {
		return nil, err
	}

	// Validate address
	if !common.IsHexAddress(req.GetAddress()) {
		return nil, errors.New("Wrong address given.")
	}

	var tx *prototype.Transaction

	if method == "stake" {
		tx, err = s.Backend.GenerateStakeTx(req.Fee, req.Address, req.Amount, req.Node)
	} else {
		tx, err = s.Backend.GenerateUnstakeTx(req.Fee, req.Address, req.Amount, req.Node)
	}

	if err != nil {
		return nil, err
	}

	res := s.createTxBodyResponse(tx)

	return res, nil
}

func (s *Server) createTxBodyResponse(tx *prototype.Transaction) *prototype.TxBodyResponse {
	var res prototype.TxBodyResponse
	res.Tx = cast.NotSignedTxValue(tx)
	return &res
}
