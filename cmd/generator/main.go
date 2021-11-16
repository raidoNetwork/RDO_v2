package main

import (
	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/raidoNetwork/RDO_v2/cmd/blockchain/flags"
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/logutil"
	"github.com/raidoNetwork/RDO_v2/shared/version"
	"github.com/raidoNetwork/RDO_v2/txgen"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"os"
	"runtime"
	runtimeDebug "runtime/debug"
)

var appFlags = []cli.Flag{
	cmd.DataDirFlag,
	cmd.LogFileName,

	flags.RPCHost,
	flags.RPCPort,
}

var log = logrus.WithField("prefix", "TxGenApp")

func main() {
	app := cli.App{}
	app.Name = "raido-tx-generator"
	app.Usage = "Raidochain transaction generator"
	app.Action = startGenerator
	app.Version = version.Version()
	app.Commands = []*cli.Command{}

	app.Flags = appFlags

	app.Before = func(ctx *cli.Context) error {
		logrus.SetFormatter(&nested.Formatter{
			HideKeys:        true,
			FieldsOrder:     []string{"component", "category"},
			TimestampFormat: "2006-01-02 15:04:05.000",
		})

		logFileName := ctx.String(cmd.LogFileName.Name)
		if logFileName != "" {
			if err := logutil.ConfigurePersistentLogging(logFileName); err != nil {
				log.WithError(err).Error("Failed to configuring logging to disk.")
			}
		}

		// runtimeDebug.SetGCPercent(100)

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

func startGenerator(ctx *cli.Context) error {
	gen, err := txgen.NewGenerator(ctx)
	if err != nil {
		return err
	}

	gen.Start()
	return nil
}
