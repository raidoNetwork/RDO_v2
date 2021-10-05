package node

import (
	"github.com/urfave/cli/v2"
	"rdo_draft/shared/cmd"
	"rdo_draft/shared/params"
	"rdo_draft/shared/tracing"
)

// configureTracing setups tracing settings for jaeger
func configureTracing(cliCtx *cli.Context) error {
	return tracing.Setup(
		"rdo-blockchain", // service name
		cliCtx.String(cmd.TracingProcessNameFlag.Name),
		cliCtx.String(cmd.TracingEndpointFlag.Name),
		cliCtx.Float64(cmd.TraceSampleFractionFlag.Name),
		cliCtx.Bool(cmd.EnableTracingFlag.Name),
	)
}

// configureChainConfig gets config from yaml file
func configureChainConfig(cliCtx *cli.Context) {
	if cliCtx.IsSet(cmd.ChainConfigFileFlag.Name) {
		chainConfigFileName := cliCtx.String(cmd.ChainConfigFileFlag.Name)
		params.LoadChainConfigFile(chainConfigFileName)
	}
}
