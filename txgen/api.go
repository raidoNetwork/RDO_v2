package txgen

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
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
}

func (c *Client) Stop() {
	c.cancel()
	c.grpcClient.Close()
}

func (c *Client) FindAllUTxO(addr string) ([]*types.UTxO, error) {
	return nil, nil
}

func (c *Client) SendTx(tx *prototype.Transaction) error {
	return nil
}

