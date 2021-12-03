package txgen

import (
	"context"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"google.golang.org/grpc"
	"time"
)

const TimeoutgRPC = 30 * time.Second

type Client struct {
	endpoint   string // address of the remote node host
	grpcClient *grpc.ClientConn
	ctx        context.Context
	cancel     context.CancelFunc

	raidoChainService  prototype.RaidoChainServiceClient
	attestationService prototype.AttestationServiceClient
}

func NewClient(endpoint string) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())

	client := Client{
		endpoint: endpoint,
		ctx:      ctx,
		cancel:   cancel,
	}

	return &client, nil
}

func (c *Client) Start() {
	ctx, _ := context.WithTimeout(c.ctx, TimeoutgRPC)
	grpcConn, err := grpc.DialContext(
		ctx,
		c.endpoint,
		grpc.WithInsecure(),
	)

	if err != nil {
		log.Errorf("cant connect to grpc: %v", err)
	}

	c.grpcClient = grpcConn

	c.raidoChainService = prototype.NewRaidoChainServiceClient(c.grpcClient)
	c.attestationService = prototype.NewAttestationServiceClient(c.grpcClient)
}

func (c *Client) Stop() {
	c.cancel()
	c.grpcClient.Close()
}

func (c *Client) FindAllUTxO(addr string) ([]*types.UTxO, error) {
	req := new(prototype.AddressRequest)
	req.Address = addr

	res, err := c.raidoChainService.GetUTxO(c.ctx, req)
	if err != nil {
		return nil, err
	}

	data := make([]*types.UTxO, len(res.GetData()))

	for i, item := range res.GetData() {
		data[i] = cast(item)
	}

	return data, nil
}

func (c *Client) FindStakeDeposits(addr string) ([]*types.UTxO, error) {
	req := new(prototype.AddressRequest)
	req.Address = addr

	res, err := c.raidoChainService.GetStakeDeposits(c.ctx, req)
	if err != nil {
		return nil, err
	}

	data := make([]*types.UTxO, len(res.GetData()))

	for i, item := range res.GetData() {
		data[i] = cast(item)
	}

	return data, nil
}

func (c *Client) SendTx(tx *prototype.Transaction) error {
	req := new(prototype.SendTxRequest)
	req.Tx = tx

	var resp *prototype.ErrorResponse
	var err error

	switch tx.Type {
	case common.NormalTxType:
		resp, err = c.attestationService.SendLegacyTx(c.ctx, req)
	case common.StakeTxType:
		resp, err = c.attestationService.SendStakeTx(c.ctx, req)
	case common.UnstakeTxType:
		resp, err = c.attestationService.SendUnstakeTx(c.ctx, req)
	default:
		return errors.New("Wrong transaction type.")
	}

	if err != nil {
		return err
	}

	if resp.GetError() != "" {
		return errors.New(resp.GetError())
	}

	return nil
}

func cast(puo *prototype.UTxO) *types.UTxO {
	return types.NewUTxO(
		puo.Hash,
		puo.From,
		puo.To,
		puo.Node,
		puo.Index,
		puo.Amount,
		puo.BlockNum,
		puo.Txtype,
		puo.Timestamp,
	)
}
