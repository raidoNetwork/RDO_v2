// Package cmd defines the command line flags for the shared utlities.
package cmd

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
	"github.com/urfave/cli/v2/altsrc"
)

var (
	// ForceClearDB removes any previously stored data at the data directory.
	ForceClearDB = &cli.BoolFlag{
		Name:  "force-clear-db",
		Usage: "Clear any previously stored data at the data directory",
	}
	// ClearDB prompts user to see if they want to remove any previously stored data at the data directory.
	ClearDB = &cli.BoolFlag{
		Name:  "clear-db",
		Usage: "Prompt for clearing any previously stored data at the data directory",
	}
	// DataDirFlag defines a path on disk.
	DataDirFlag = &cli.StringFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keystore",
		Value: DefaultDataDir(),
	}
	// LogFileName specifies the log output file name.
	LogFileName = &cli.StringFlag{
		Name:  "log-file",
		Usage: "Specify log file name, relative or absolute",
	}
	// ConfigFileFlag specifies the filepath to load flag values.
	ConfigFileFlag = &cli.StringFlag{
		Name:  "config-file",
		Usage: "The filepath to a yaml file with flag values",
	}
	// ChainConfigFileFlag specifies the filepath to load flag values.
	ChainConfigFileFlag = &cli.StringFlag{
		Name:  "chain-config-file",
		Usage: "The path to a YAML file with chain config values",
	}
	// GrpcMaxCallRecvMsgSizeFlag defines the max call message size for GRPC
	GrpcMaxCallRecvMsgSizeFlag = &cli.IntFlag{
		Name:  "grpc-max-msg-size",
		Usage: "Integer to define max recieve message call size (default: 4194304 (for 4MB))",
		Value: 1 << 22,
	}
	// BoltMMapInitialSizeFlag specifies the initial size in bytes of boltdb's mmap syscall.
	BoltMMapInitialSizeFlag = &cli.IntFlag{
		Name:  "bolt-mmap-initial-size",
		Usage: "Specifies the size in bytes of bolt db's mmap syscall allocation",
		Value: 536870912, // 512 Mb as a default value.
	}

	// EnableTracingFlag defines a flag to enable p2p message tracing.
	EnableTracingFlag = &cli.BoolFlag{
		Name:  "enable-tracing",
		Usage: "Enable request tracing.",
	}
	// TracingProcessNameFlag defines a flag to specify a process name.
	TracingProcessNameFlag = &cli.StringFlag{
		Name:  "tracing-process-name",
		Usage: "The name to apply to tracing tag \"process_name\"",
		Value: "raido-trace",
	}
	// TracingEndpointFlag flag defines the http endpoint for serving traces to Jaeger.
	TracingEndpointFlag = &cli.StringFlag{
		Name:  "tracing-endpoint",
		Usage: "Tracing endpoint defines where raido chain traces are exposed to Jaeger.",
		Value: "http://127.0.0.1:14268/api/traces",
	}
	// TraceSampleFractionFlag defines a flag to indicate what fraction of p2p
	// messages are sampled for tracing.
	TraceSampleFractionFlag = &cli.Float64Flag{
		Name:  "trace-sample-fraction",
		Usage: "Indicate what fraction of p2p messages are sampled for tracing.",
		Value: 0.20,
	}
)

// LoadFlagsFromConfig sets flags values from config file if ConfigFileFlag is set.
func LoadFlagsFromConfig(cliCtx *cli.Context, flags []cli.Flag) error {
	if cliCtx.IsSet(ConfigFileFlag.Name) {
		if err := altsrc.InitInputSourceWithContext(flags, altsrc.NewYamlSourceFromFlagFunc(ConfigFileFlag.Name))(cliCtx); err != nil {
			return err
		}
	}
	return nil
}

// ValidateNoArgs insures that the application is not run with erroneous arguments or flags.
// This function should be used in the app.Before, whenever the application supports a default command.
func ValidateNoArgs(ctx *cli.Context) error {
	commandList := ctx.App.Commands
	parentCommand := ctx.Command
	isParamForFlag := false
	for _, a := range ctx.Args().Slice() {
		// We don't validate further if
		// the following value is actually
		// a parameter for a flag.
		if isParamForFlag {
			isParamForFlag = false
			continue
		}
		if strings.HasPrefix(a, "-") || strings.HasPrefix(a, "--") {
			// In the event our flag doesn't specify
			// the relevant argument with an equal
			// sign, we can assume the next argument
			// is the relevant value for the flag.
			flagName := strings.TrimPrefix(a, "--")
			flagName = strings.TrimPrefix(flagName, "-")
			if !strings.Contains(a, "=") && !isBoolFlag(parentCommand, flagName) {
				isParamForFlag = true
			}
			continue
		}
		c := checkCommandList(commandList, a)
		if c == nil {
			return fmt.Errorf("unrecognized argument: %s", a)
		}
		// Set the command list as the subcommand's
		// from the current selected parent command.
		commandList = c.Subcommands
		parentCommand = c
	}
	return nil
}

// verifies that the provided command is in the command list.
func checkCommandList(commands []*cli.Command, name string) *cli.Command {
	for _, c := range commands {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func isBoolFlag(com *cli.Command, name string) bool {
	for _, f := range com.Flags {
		switch bFlag := f.(type) {
		case *cli.BoolFlag:
			if bFlag.Name == name {
				return true
			}
		case *altsrc.BoolFlag:
			if bFlag.Name == name {
				return true
			}
		}
	}
	return false
}
