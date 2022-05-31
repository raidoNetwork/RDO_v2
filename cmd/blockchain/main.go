package main

import (
	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/node"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/version"
	"github.com/raidoNetwork/RDO_v2/utils/logger"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"runtime"
	runtimeDebug "runtime/debug"
)

var appFlags = []cli.Flag{
	// RPC flags
	flags.RPCHost,
	flags.RPCPort,

	// gRPC gateway flags
	flags.GRPCGatewayHost,
	flags.GRPCGatewayPort,
	flags.GPRCGatewayCorsDomain,

	// data storage directories
	cmd.DataDirFlag,
	cmd.LogFileName,
	cmd.VerbosityFlag,

	// config flags
	cmd.ConfigFileFlag,
	cmd.ChainConfigFileFlag,

	// db flags
	cmd.ClearDB,
	cmd.ForceClearDB,

	// SQL config
	cmd.SQLConfigPath,

	// p2p
	flags.P2PHost,
	flags.P2PPort,
	flags.P2PBootstrapNodes,

	// metrics
	flags.EnableMetrics,
	flags.MetricsHost,
	flags.MetricsPort,
}

var log = logrus.WithField("prefix", "main")

func main() {
	app := cli.App{}
	app.Name = "raido-chain"
	app.Usage = "Raido blockchain"
	app.Action = startNode
	app.Version = version.Version()
	app.Commands = []*cli.Command{}

	app.Flags = appFlags

	app.Before = func(ctx *cli.Context) error {
		// Load cmd from config file, if specified.
		if err := cmd.LoadFlagsFromConfig(ctx, app.Flags); err != nil {
			return err
		}

		logrus.SetFormatter(&nested.Formatter{
			HideKeys:        true,
			FieldsOrder:     []string{"component", "category"},
			TimestampFormat: "2006-01-02 15:04:05.000",
		})

		verbosity := ctx.String(cmd.VerbosityFlag.Name)
		level, err := logrus.ParseLevel(verbosity)
		if err != nil {
			return err
		}
		logrus.SetLevel(level)

		logFileName := ctx.String(cmd.LogFileName.Name)
		if logFileName != "" {
			if err := logger.ConfigurePersistentLogging(logFileName); err != nil {
				log.WithError(err).Error("Failed to configuring logging to disk.")
			}
		}

		runtimeDebug.SetGCPercent(100)
		runtime.GOMAXPROCS(runtime.NumCPU())

		return cmd.ValidateNoArgs(ctx)
	}

	defer func() {
		if x := recover(); x != nil {
			log.Errorf("Runtime panic: %v\n%v", x, string(runtimeDebug.Stack()))
			panic(x)
		}
	}()

	if err := app.Run(os.Args); err != nil {
		log.Error(err.Error())
	}
}

func startNode(ctx *cli.Context) error {
	rdo, err := node.New(ctx)
	if err != nil {
		return errors.Wrap(err, "Node start error")
	}

	rdo.Start()
	return nil
}
