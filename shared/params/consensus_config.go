package params

import "github.com/mohae/deepcopy"

var consensusConfig = mainnetConsensusConfig()

type PoAConfig struct {
	Proposers []string `yaml:"proposers"`
	CommitteeSize int `yaml:"committee-size"`
}

func mainnetConsensusConfig() *PoAConfig {
	return &PoAConfig{
		Proposers: []string{
			"0xfe9353d875707a028ca049d776256da27f2c2359", // main node
			"0x0290896c2fe347db4d1971d95414cafc641b21f0", // node 1
			"0x8da19d2ef6b876900b214133e48469467fb85342", // node 2
		},
		CommitteeSize: 2,
	}
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