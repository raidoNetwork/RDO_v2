package rdochain

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/gateway"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

var log = logrus.WithField("prefix", "RPC Server")

type Server struct {
	Server       *grpc.Server
	ChainService gateway.ChainAPI

	prototype.RaidoChainServiceServer
}

func (s *Server) GetUTxO(ctx context.Context, request *prototype.AddressRequest) (*prototype.UTxOResponse, error) {
	err := request.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetUTxO error: %s", err)
		return nil, err
	}

	addr := request.GetAddress()

	log.Infof("ChainAPI.GetUTxO %s", addr)

	arr, err := s.ChainService.FindAllUTxO(addr)
	if err != nil {
		return nil, err
	}

	response := new(prototype.UTxOResponse)
	response.Data = make([]*prototype.UTxO, len(arr))

	for i, uo := range arr {
		response.Data[i] = convertProtoToInner(uo)
	}

	return response, nil
}

func (s *Server) GetStatus(ctx context.Context, nothing *emptypb.Empty) (*prototype.StatusResponse, error) {
	res := new(prototype.StatusResponse)

	data, err := s.ChainService.GetSyncStatus()
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

	log.Infof("ChainAPI.GetBlockByNum(%d)", req.GetNum())

	block, err := s.ChainService.GetBlockByNum(req.GetNum())
	if err != nil {
		res.Error = err.Error()
		return res, err
	}

	if block == nil {
		err = errors.Errorf("Not found block with num %d", req.GetNum())
		res.Error = err.Error()
		return res, err
	}

	res.Block = convBlock(block)

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

	block, err := s.ChainService.GetBlockByHash(req.GetHash())
	if err != nil {
		res.Error = err.Error()
		return res, err
	}

	if block == nil {
		err = errors.Errorf("Not found block with hash %s", req.GetHash())
		res.Error = err.Error()
		return res, err
	}

	res.Block = convBlock(block)

	return res, nil
}

func (s *Server) GetBalance(ctx context.Context, req *prototype.AddressRequest) (*prototype.BalanceResponse, error) {
	res := new(prototype.BalanceResponse)
	err := req.Validate()
	if err != nil {
		log.Errorf("ChainAPI.GetBalance error: %s", err)
		res.Error = err.Error()
		return nil, err
	}

	addr := req.GetAddress()
	log.Infof("ChainAPI.GetBalance(%s)", addr)

	balance, err := s.ChainService.GetBalance(addr)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}

	res.Balance = balance

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

	tx, err := s.ChainService.GetTransaction(req.GetHash())
	if err != nil {
		res.Error = err.Error()
		return res, err
	}

	if tx == nil {
		err = errors.Errorf("Not found block with hash %s", req.GetHash())
		res.Error = err.Error()
		return res, err
	}

	res.Tx = convTx(tx)

	return res, nil
}
