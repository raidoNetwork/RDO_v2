package params

import (
	"time"
)

// MainnetConfig returns the configuration to be used in the main network.
func MainnetConfig() *RDOBlockChainConfig {
	return mainnetRDOConfig
}

// UseMainnetConfig for rdo chain services.
func UseMainnetConfig() {
	raidoConfig = MainnetConfig()
}

var mainnetRDOConfig = &RDOBlockChainConfig{
	// Initial value constants.
	ZeroHash: [32]byte{},

	DefaultBufferSize:        10000,
	RPCSyncCheck:             1,
	EmptySignature:           [96]byte{},
	DefaultPageSize:          250,
	MaxPeersToSync:           15,
	GenesisCountdownInterval: time.Minute,
}
