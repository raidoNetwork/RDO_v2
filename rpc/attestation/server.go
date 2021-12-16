package attestation

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/rpc/api"
	"github.com/raidoNetwork/RDO_v2/rpc/cast"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var log = logrus.WithField("prefix", "RPC AttestationServer")

var ErrWrongTxType = errors.New("Wrong transaction type.")

type Server struct {
	Server  *grpc.Server
	Backend api.AttestationAPI

	prototype.UnimplementedAttestationServer
}

func (s *Server) sendTx(request *prototype.SendTxRequest, txType uint32, method string) (*prototype.ErrorResponse, error) {
	txv := request.GetTx()
	resp := new(prototype.ErrorResponse)

	err := txv.Validate()
	if err != nil {
		resp.Error = err.Error()
		return resp, status.Error(17, err.Error())
	}

	tx := cast.TxFromTxValue(txv)
	if tx == nil {
		err := errors.New("Nil tx given.")
		resp.Error = err.Error()
		return resp, err
	}

	err = tx.Validate()
	if err != nil {
		resp.Error = err.Error()
		return resp, status.Error(17, err.Error())
	}

	// check tx type
	if tx.Type != txType {
		resp.Error = ErrWrongTxType.Error()
		return resp, ErrWrongTxType
	}

	log.Infof("AttestationAPI.%s %s", method, common.BytesToHash(tx.Hash).Hex())

	err = s.Backend.SendRawTx(tx)
	if err != nil {
		resp.Error = err.Error()
		return resp, err
	}

	return resp, nil
}

func (s *Server) SendLegacyTx(ctx context.Context, request *prototype.SendTxRequest) (*prototype.ErrorResponse, error) {
	return s.sendTx(request, common.NormalTxType, "SendLegacyTx")
}

func (s *Server) SendStakeTx(ctx context.Context, request *prototype.SendTxRequest) (*prototype.ErrorResponse, error) {
	return s.sendTx(request, common.StakeTxType, "SendStakeTx")
}

func (s *Server) SendUnstakeTx(ctx context.Context, request *prototype.SendTxRequest) (*prototype.ErrorResponse, error) {
	return s.sendTx(request, common.UnstakeTxType, "SendUnstakeTx")
}

func (s *Server) SendRawTx(ctx context.Context, request *prototype.RawTxRequest) (*prototype.ErrorResponse, error) {
	if len(request.Data) == 0 {
		return nil, status.Error(17, "Empty request given.")
	}

	resp := new(prototype.ErrorResponse)

	tx, err := UnmarshalTx(request.Data)
	if err != nil {
		resp.Error = err.Error()
		return resp, status.Error(17, err.Error())
	}

	err = s.Backend.SendRawTx(tx)
	if err != nil {
		resp.Error = err.Error()
		return resp, err
	}

	return resp, nil
}

func (s *Server) GetFee(ctx context.Context, empty *emptypb.Empty) (*prototype.NumberResponse, error) {
	log.Info("AttestationAPI.GetFee")

	res := new(prototype.NumberResponse)
	res.Result = s.Backend.GetFee()

	return res, nil
}

func (s *Server) GetPendingTransactions(ctx context.Context, empty *emptypb.Empty) (*prototype.TransactionsResponse, error) {
	log.Info("AttestationAPI.GetPendingTransactions")

	response := new(prototype.TransactionsResponse)
	txBatch, err := s.Backend.GetPendingTransactions()
	if err != nil {
		response.Error = err.Error()
		return response, err
	}

	response.Tx = make([]*prototype.TxValue, len(txBatch))
	for i, tx := range txBatch {
		response.Tx[i] = cast.TxValue(tx)
	}

	return response, nil
}
