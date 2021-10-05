package flags

import (
	"github.com/urfave/cli/v2"
)

var (
	// RPCHost defines the host on which the RPC server should listen.
	RPCHost = &cli.StringFlag{
		Name:  "rpc-host",
		Usage: "Host on which the RPC server should listen",
		Value: "127.0.0.1",
	}
	// RPCPort defines a rdo node RPC port to open.
	RPCPort = &cli.IntFlag{
		Name:  "rpc-port",
		Usage: "RPC port exposed by a rdo node",
		Value: 4000,
	}
	// MonitoringPortFlag defines the http port used to serve prometheus metrics.
	MonitoringPortFlag = &cli.IntFlag{
		Name:  "monitoring-port",
		Usage: "Port used to listening and respond metrics for prometheus.",
		Value: 8080,
	}
	// CertFlag defines a flag for the node's TLS certificate.
	CertFlag = &cli.StringFlag{
		Name:  "tls-cert",
		Usage: "Certificate for secure gRPC. Pass this and the tls-key flag in order to use gRPC securely.",
	}
	// KeyFlag defines a flag for the node's TLS key.
	KeyFlag = &cli.StringFlag{
		Name:  "tls-key",
		Usage: "Key for secure gRPC. Pass this and the tls-cert flag in order to use gRPC securely.",
	}
	// GRPCGatewayHost specifies a gRPC gateway host for RDO.
	GRPCGatewayHost = &cli.StringFlag{
		Name:  "grpc-gateway-host",
		Usage: "The host on which the gateway server runs on",
		Value: "127.0.0.1",
	}
	// GRPCGatewayPort specifies a gRPC gateway port for RDO.
	GRPCGatewayPort = &cli.IntFlag{
		Name:  "grpc-gateway-port",
		Usage: "The port on which the gateway server runs on",
		Value: 3500,
	}
	// GPRCGatewayCorsDomain serves preflight requests when serving gRPC JSON gateway.
	GPRCGatewayCorsDomain = &cli.StringFlag{
		Name: "grpc-gateway-corsdomain",
		Usage: "Comma separated list of domains from which to accept cross origin requests " +
			"(browser enforced). This flag has no effect if not used with --grpc-gateway-port.",
		Value: "http://localhost:4200,http://localhost:7500,http://127.0.0.1:4200,http://127.0.0.1:7500,http://0.0.0.0:4200,http://0.0.0.0:7500",
	}
	// MinSyncPeers specifies the required number of successful peer handshakes in order
	// to start syncing with external peers.
	MinSyncPeers = &cli.IntFlag{
		Name:  "min-sync-peers",
		Usage: "The required number of valid peers to connect with before syncing.",
		Value: 3,
	}
	// HeadSync starts the rdo node from the previously saved head state and syncs from there.
	HeadSync = &cli.BoolFlag{
		Name:  "head-sync",
		Usage: "Starts the rdo node with the previously saved head state instead of finalized state.",
	}
	// BlockBatchLimit specifies the requested block batch size.
	BlockBatchLimit = &cli.IntFlag{
		Name:  "block-batch-limit",
		Usage: "The amount of blocks the local peer is bounded to request and respond to in a batch.",
		Value: 64,
	}
	// BlockBatchLimitBurstFactor specifies the factor by which block batch size may increase.
	BlockBatchLimitBurstFactor = &cli.IntFlag{
		Name:  "block-batch-limit-burst-factor",
		Usage: "The factor by which block batch limit may increase on burst.",
		Value: 10,
	}
	// DisableSync disables a node from syncing at start-up. Instead the node enters regular sync
	// immediately.
	DisableSync = &cli.BoolFlag{
		Name:  "disable-sync",
		Usage: "Starts the rdo node without entering initial sync and instead exits to regular sync immediately.",
	}
	// ChainID defines a flag to set the chain id. If none is set, it derives this value from NetworkConfig
	ChainID = &cli.Uint64Flag{
		Name:  "chain-id",
		Usage: "Sets the chain id of the raido blockchain",
	}
	// NetworkID defines a flag to set the network id. If none is set, it derives this value from NetworkConfig
	NetworkID = &cli.Uint64Flag{
		Name:  "network-id",
		Usage: "Sets the network id of the raido blockchain.",
	}

	// GenesisStatePath defines a flag to start the raido blockchain from a give genesis state file.
	GenesisStatePath = &cli.StringFlag{
		Name:  "genesis-state",
		Usage: "Load a genesis state from ssz file. ",
	}

	// DBWriteTest defines flag to switch DB service keys
	DBWriteTest = &cli.BoolFlag{
		Name: "db-write-test",
		Usage: "Switch DB service to using block keys = Hash(num + suffix)." +
			"Use this flag before ",
	}

	// DBReadTest defines flag to start read db test
	DBReadTest = &cli.BoolFlag{
		Name: "db-read-test",
		Usage: "Starts database read test. Read all block and than get random blocks by num" +
			" with collecting statistics.",
	}

	DBStats = &cli.BoolFlag{
		Name:  "db-stats",
		Usage: "Show database blocks number.",
	}

	LanSrv = &cli.BoolFlag{
		Name:  "lan-srv",
		Usage: "Starts lan service",
	}

	LanSrvStat = &cli.BoolFlag{
		Name:  "lan-srv-stat",
		Usage: "Show full statistics of lan server",
	}
)
