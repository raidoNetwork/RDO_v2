package flags

import (
	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

var (
	// RPCHost defines the host on which the RPC server should listen.
	RPCHost = altsrc.NewStringFlag(&cli.StringFlag{
		Name:  "rpc-host",
		Usage: "Host on which the RPC server should listen",
		Value: "127.0.0.1",
	})
	// RPCPort defines a rdo node RPC port to open.
	RPCPort = altsrc.NewIntFlag(&cli.IntFlag{
		Name:  "rpc-port",
		Usage: "RPC port exposed by a rdo node",
		Value: 4000,
	})
	// GRPCGatewayHost specifies a gRPC gateway host for RDO.
	GRPCGatewayHost = altsrc.NewStringFlag(&cli.StringFlag{
		Name:  "grpc-gateway-host",
		Usage: "The host on which the gateway server runs on",
		Value: "127.0.0.1",
	})
	// GRPCGatewayPort specifies a gRPC gateway port for RDO.
	GRPCGatewayPort = altsrc.NewIntFlag(&cli.IntFlag{
		Name:  "grpc-gateway-port",
		Usage: "The port on which the gateway server runs on",
		Value: 5555,
	})
	// GPRCGatewayCorsDomain serves preflight requests when serving gRPC JSON gateway.
	GPRCGatewayCorsDomain = altsrc.NewStringFlag(&cli.StringFlag{
		Name: "grpc-gateway-corsdomain",
		Usage: "Comma separated list of domains from which to accept cross origin requests " +
			"(browser enforced). This flag has no effect if not used with --grpc-gateway-port.",
		Value: "http://localhost:4200, http://localhost:5555,http://localhost:7500,http://127.0.0.1:4200,http://127.0.0.1:5555,http://0.0.0.0:4200,http://0.0.0.0:5555",
	})
	// P2PPort specifies a p2p port.
	P2PPort = altsrc.NewIntFlag(&cli.IntFlag{
		Name:  "p2p-port",
		Usage: "P2P service port to listen",
		Value: 9999,
	})
	// P2PHost specifies a p2p host.
	P2PHost = altsrc.NewStringFlag(&cli.StringFlag{
		Name:  "p2p-host",
		Usage: "P2P service host",
		Value: "127.0.0.1",
	})
	// P2PBootstrapNodes first nodes to connect
	P2PBootstrapNodes = altsrc.NewStringSliceFlag(&cli.StringSliceFlag{
		Name: "p2p-bootstrap-nodes",
		Usage: "P2P nodes addresses for initial connections",
		Value: cli.NewStringSlice(),
	})
	P2PEnableNat = altsrc.NewBoolFlag(&cli.BoolFlag{
		Name: "p2p-nat",
		Usage: "Enable NAT support for P2P",
		Value: false,
	})
	EnableMetrics = altsrc.NewBoolFlag(&cli.BoolFlag{
		Name: "enable-metrics",
		Usage: "Enable Prometheus metric server",
		Value: false,
	})
	MetricsHost = altsrc.NewStringFlag(&cli.StringFlag{
		Name: "metrics-host",
		Usage: "The host on which metrics endpoint should listen",
		Value: "127.0.0.1",
	})
	MetricsPort = altsrc.NewIntFlag(&cli.IntFlag{
		Name:  "metrics-port",
		Usage: "Metrics endpoint port",
		Value: 2121,
	})
	// DisableSync enable syncing with network
	DisableSync = altsrc.NewBoolFlag(&cli.BoolFlag{
		Name: "disable-sync",
		Usage: "Disable syncing with network",
		Value: false,
	})
	// MinSyncPeers specifies sync peers minimum
	MinSyncPeers = altsrc.NewIntFlag(&cli.IntFlag{
		Name:  "min-sync-peers",
		Usage: "Minimal count of peers for syncing",
		Value: 1,
	})
)
