package params

import "github.com/mohae/deepcopy"

var consensusConfig = mainnetConsensusConfig()

type PoAConfig struct {
	Proposers []string `yaml:"proposers"`
}

func mainnetConsensusConfig() *PoAConfig {
	return &PoAConfig{}
}

func ConsensusConfig() *PoAConfig {
	return consensusConfig
}

func OverwriteConsensusConfig(cfg *PoAConfig) {
	consensusConfig = cfg
}

// Copy returns a copy of the config object.
func (c *PoAConfig) Copy() *PoAConfig {
	config, ok := deepcopy.Copy(*c).(PoAConfig)
	if !ok {
		config = *consensusConfig
	}
	return &config
}