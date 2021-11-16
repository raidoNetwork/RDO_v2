package rdochain

import (
	"context"
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