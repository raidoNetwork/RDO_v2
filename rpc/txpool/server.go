package txpool

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/gateway"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

type Server struct {
	Server             *grpc.Server
	AttestationService gateway.AttestationAPI

	prototype.UnimplementedAttestationServiceServer
}

func (s *Server) SendTx(ctx context.Context, request *prototype.SendTxRequest) (*prototype.ErrorResponse, error) {
	tx := request.GetTx()
	resp := new(prototype.ErrorResponse)

	if tx == nil {
		err := errors.New("Nil tx given.")
		resp.Error = err.Error()
		return resp, err
	}

	err := tx.Validate()
	if err != nil {
		resp.Error = err.Error()
		return resp, err
	}

	log.Infof("Got transaction %s", common.BytesToHash(tx.Hash).Hex())

	err = s.AttestationService.SendRawTx(tx)
	if err != nil {
		resp.Error = err.Error()
		return resp, err
	}

	return resp, nil
}
