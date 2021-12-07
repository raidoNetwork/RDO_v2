package params

// RDOBlockChainConfig contains constant configs for node to participate in raido blockchain.
type RDOBlockChainConfig struct {
	SlotTime int64 `yaml:"SLOT_TIME"` // SlotTime setups block generator timeout.

	// Reward constant
	RewardBase uint64 `yaml:"REWARD_BASE"` // RewardBase is used to calculate the per block reward. RewardBase use Groi.
	MinimalFee uint64 `yaml:"MINIMAL_FEE"` // MinimalFee defines the minimal fee per transaction byte.
	BlockSize  int    `yaml:"BLOCK_SIZE"`  // BlockSize defines block size limit.

	// RDO constants
	RoiPerRdo  uint64 // RoiPerRdo is the amount of roi corresponding to 1 RDO.
	KroiPerRdo uint64 // KroiPerRdo is the amount of Groi = 1000 roi corresponding to 1 RDO.

	// Stake config
	ValidatorRegistryLimit int  `yaml:"VALIDATOR_REGISTRY_LIMIT"` // ValidatorRegistryLimit defines the maximum count of validators can participate in rdochain.
	StakeSlotUnit          uint64 `yaml:"STAKE_SLOT_UNIT"`          // StakeSlotUnit defines the amount of RDO needed to fill one stake slot.

	GenesisPath string `yaml:"GENESIS_PATH"` // GenesisPath defines path to the Genesis JSON file.
}
