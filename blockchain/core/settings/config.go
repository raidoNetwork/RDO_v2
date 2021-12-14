package settings

import (
	"github.com/raidoNetwork/RDO_v2/shared/cmd"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/urfave/cli/v2"
)

// ConfigureChainConfig gets config from yaml file
func ConfigureChainConfig(cliCtx *cli.Context) {
	if cliCtx.IsSet(cmd.ChainConfigFileFlag.Name) {
		chainConfigFileName := cliCtx.String(cmd.ChainConfigFileFlag.Name)
		params.LoadChainConfigFile(chainConfigFileName)
	}
}
