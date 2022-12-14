package params

// MainnetConfig returns the configuration to be used in the main network.
func MainnetConfig() *RDOBlockChainConfig {
	return mainnetRDOConfig
}

// UseMainnetConfig for rdo chain services.
func UseMainnetConfig() {
	raidoConfig = MainnetConfig()
}

var mainnetRDOConfig = &RDOBlockChainConfig{
	SlotTime:        7, // 7 seconds
	RewardBase:      1100000,
	StakeSlotUnit:   5000,
	MinimalFee:      1, // 1 roi
	RoiPerRdo:       1e8,
	GenesisPath:     "",
	BlockSize:       300 * 1024, // 300 kB
	ResponseTimeout: 15,
	SlotsPerEpoch:   200,
	NTPPool:         "pool.ntp.org",
	NTPChecks:       3,
	NTPThreshold:    1200,
}
