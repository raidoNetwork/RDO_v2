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
	SlotTime:               7,
	RewardBase:             11,
	MinimalFee:             1,
	RoiPerRdo:              100000000,
	KroiPerRdo:             100000,
	ValidatorRegistryLimit: 100,
	StakeSlotUnit:          5000,
	GenesisPath:            "",
	BlockSize:              300 * 1024,
}
