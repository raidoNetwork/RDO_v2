package params

import (
	"github.com/mohae/deepcopy"
)

var raidoConfig = MainnetConfig()

// RaidoConfig retrieves rdo chain config.
func RaidoConfig() *RDOBlockChainConfig {
	return raidoConfig
}

// OverrideRDOConfig by replacing the config. The preferred pattern is to
// call RaidoConfig(), change the specific parameters, and then call
// OverrideRDOConfig(c). Any subsequent calls to params.RDOConfig() will
// return this new configuration.
func OverrideRDOConfig(c *RDOBlockChainConfig) {
	raidoConfig = c
}

// Copy returns a copy of the config object.
func (c *RDOBlockChainConfig) Copy() *RDOBlockChainConfig {
	config, ok := deepcopy.Copy(*c).(RDOBlockChainConfig)
	if !ok {
		config = *raidoConfig
	}
	return &config
}
