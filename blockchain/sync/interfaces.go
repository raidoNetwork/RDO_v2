package sync

import (
	"context"
	ssz "github.com/ferranbt/fastssz"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/raidoNetwork/RDO_v2/events"
	"github.com/raidoNetwork/RDO_v2/p2p"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"io"
)

type GossipPublisher interface{
	Publish(string, []byte) error
	Notifier() *events.Bus
}

type ValidatorGossipPublisher interface {
	ValidatorNotifier() *events.Bus
}

type BlockchainInfo interface{
	GetBlockCount() uint64
	GetHeadBlockNum() uint64
	GetHeadBlock() (*prototype.Block, error)
	GenesisHash() common.Hash
	GetGenesis() *prototype.Block
	GetBlocksRange(context.Context, uint64, uint64) ([]*prototype.Block, error)
}

type BlockStorage interface {
	FinalizeBlock(*prototype.Block) error
}

type StreamProcessor interface {
	SetStreamHandler(string, network.StreamHandler)
	DecodeStream(io.Reader, ssz.Unmarshaler) error
	EncodeStream(io.Writer, ssz.Marshaler) (int, error)
	CreateStream(context.Context, ssz.Marshaler, string, peer.ID) (network.Stream, error)
}

type P2P interface {
	PeerStore() *p2p.PeerStore
	AddConnectionHandlers(connectHandler, disconnectHandler p2p.ConnectionHandler)
	GossipPublisher
	StreamProcessor
	ValidatorGossipPublisher
}
