package rdochain

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/rpc/api"
	"github.com/raidoNetwork/RDO_v2/rpc/cast"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

var log = logrus.WithField("prefix", "RPC ChainServer")

type Server struct {
	Server      *grpc.Server
	Backend     api.ChainAPI
	Attestation api.AttestationAPI

	prototype.UnimplementedRaidoChainServer
}

func (s *Server) GetUTxO(ctx context.Context, request *prototype.AddressRequest) (*prototype.UTxOResponse, error) {
	err := request.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetUTxO error: %s", err)
		return nil, err
	}

	addr := request.GetAddress()

	log.Infof("ChainAPI.GetUTxO(%s)", addr)

	arr, err := s.Backend.FindAllUTxO(addr)
	if err != nil {
		return nil, err
	}

	response := new(prototype.UTxOResponse)
	response.Data = make([]*prototype.UTxO, len(arr))

	for i, uo := range arr {
		response.Data[i] = cast.ProtoUTxO(uo)
	}

	return response, nil
}

func (s *Server) GetStatus(ctx context.Context, nothing *emptypb.Empty) (*prototype.StatusResponse, error) {
	res := new(prototype.StatusResponse)

	data, err := s.Backend.GetSyncStatus()
	if err != nil {
		res.Error = err.Error()
		return res, err
	}

	res.Data = data

	return res, nil
}

func (s *Server) GetBlockByNum(ctx context.Context, req *prototype.NumRequest) (*prototype.BlockResponse, error) {
	res := new(prototype.BlockResponse)

	err := req.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetBlockByNum error: %s", err)
		res.Error = err.Error()
		return res, err
	}

	var block *prototype.Block
	if req.GetNum() == "latest" {
		block, err = s.Backend.GetLatestBlock()
	} else {
		var num uint64

		num, err = strconv.ParseUint(req.GetNum(), 10, 64)
		if err != nil {
			return nil, status.Error(3, "Wrong number given.")
		}

		log.Infof("ChainAPI.GetBlockByNum(%d)", num)

		block, err = s.Backend.GetBlockByNum(num)
	}

	// error getting block
	if err != nil {
		return nil, err
	}

	if block == nil {
		err = errors.Errorf("Not found block with num %s", req.GetNum())
		res.Error = err.Error()
		return res, err
	}

	res.Block = cast.BlockValue(block)

	return res, nil
}

func (s *Server) GetBlockByHash(ctx context.Context, req *prototype.HashRequest) (*prototype.BlockResponse, error) {
	res := new(prototype.BlockResponse)

	err := req.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetBlockByHash error: %s", err)
		res.Error = err.Error()
		return res, err
	}

	log.Infof("ChainAPI.GetBlockByHash(%s)", req.GetHash())

	block, err := s.Backend.GetBlockByHashHex(req.GetHash())
	if err != nil {
		res.Error = err.Error()
		return res, err
	}

	if block == nil {
		err = errors.Errorf("Not found block with hash %s", req.GetHash())
		res.Error = err.Error()
		return res, err
	}

	res.Block = cast.BlockValue(block)

	return res, nil
}

func (s *Server) GetBalance(ctx context.Context, req *prototype.AddressRequest) (*prototype.NumberResponse, error) {
	res := new(prototype.NumberResponse)
	err := req.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetBalance error: %s", err)
		res.Error = err.Error()
		return nil, err
	}

	addr := req.GetAddress()
	log.Infof("ChainAPI.GetBalance(%s)", addr)

	balance, err := s.Backend.GetBalance(addr)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}

	res.Result = balance

	return res, nil
}

func (s *Server) GetTransaction(ctx context.Context, req *prototype.HashRequest) (*prototype.TransactionResponse, error) {
	res := new(prototype.TransactionResponse)

	err := req.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetTransaction error: %s", err)
		res.Error = err.Error()
		return res, err
	}

	log.Infof("ChainAPI.GetTransaction(%s)", req.GetHash())

	tx, err := s.Backend.GetTransaction(req.GetHash())
	if err != nil {
		res.Error = err.Error()
		return res, err
	}

	if tx == nil {
		err = errors.Errorf("Not found transaction %s", req.GetHash())
		res.Error = err.Error()
		return res, err
	}

	res.Tx = cast.TxValue(tx)

	return res, nil
}

func (s *Server) GetStakeDeposits(ctx context.Context, request *prototype.AddressRequest) (*prototype.UTxOResponse, error) {
	err := request.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetStakeDeposits error: %s", err)
		return nil, err
	}

	addr := request.GetAddress()

	log.Infof("ChainAPI.GetStakeDeposits(%s)", addr)

	arr, err := s.Backend.GetStakeDeposits(addr, "all")
	if err != nil {
		return nil, err
	}

	response := new(prototype.UTxOResponse)
	response.Data = make([]*prototype.UTxO, len(arr))

	for i, uo := range arr {
		response.Data[i] = cast.ProtoUTxO(uo)
	}

	return response, nil
}

func (s *Server) GetTransactionsCount(ctx context.Context, request *prototype.AddressRequest) (*prototype.NumberResponse, error) {
	err := request.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetTransactionsCount error: %s", err)
		return nil, err
	}

	addr := request.GetAddress()

	log.Infof("ChainAPI.GetTransactionsCount %s", addr)

	response := new(prototype.NumberResponse)
	nonce, err := s.Backend.GetTransactionsCountHex(addr)
	if err != nil {
		response.Error = err.Error()
		return response, err
	}

	response.Result = nonce

	return response, nil
}

// GetBlocksStartCount returns a number of blocks starting at some index.
// Start parameter can be negative, signifying counting backwards.
func (s *Server) GetBlocksStartCount(ctx context.Context, request *prototype.BlocksStartCountRequest) (*prototype.BlocksStartCountResponse, error) {
	response := new(prototype.BlocksStartCountResponse)

	err := request.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetBlocksStartCount error: %s", err)
		response.Error = err.Error()
		return response, err
	}

	start := request.GetStart()
	limit := request.GetLimit()

	blocks, err := s.Backend.GetBlocksStartCount(start, limit)
	if err != nil {
		response.Error = err.Error()
		return response, err
	}

	blockValues := make([]*prototype.BlockValue, 0)
	for _, b := range blocks {
		blockValues = append(blockValues, cast.BlockValue(b))
	}

	response.Blocks = blockValues
	return response, nil
}

// ListValidators return validators (with reserved slots) that can be staked on.
func (s *Server) ListValidators(ctx context.Context, nothing *emptypb.Empty) (*prototype.ValidatorAddressesResponse, error) {
	response := new(prototype.ValidatorAddressesResponse)
	validators := s.Attestation.ListValidators()

	response.Nodes = validators
	return response, nil
}

// ListStakeValidators return validators (with reserved slots) that can be staked on.
func (s *Server) ListStakeValidators(ctx context.Context, nothing *emptypb.Empty) (*prototype.ValidatorAddressesResponse, error) {
	response := new(prototype.ValidatorAddressesResponse)
	validators := s.Attestation.ListStakeValidators()

	response.Nodes = validators
	return response, nil
}
