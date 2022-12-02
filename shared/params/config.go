package params

// RDOBlockChainConfig contains constant configs for node to participate in raido blockchain.
type RDOBlockChainConfig struct {
	SlotTime int64 `yaml:"SLOT_TIME"` // SlotTime setups block generator timeout.

	// Reward constant
	RewardBase     uint64 `yaml:"REWARD_BASE"`     // RewardBase is used to calculate the per block reward. To define RewardBase use roi unit.
	ProposerReward uint64 `yaml:"PROPOSER_REWARD"` // ProposerReward define the reward amount for block proposer.
	MinimalFee     uint64 `yaml:"MINIMAL_FEE"`     // MinimalFee defines the minimal fee per transaction byte.
	BlockSize      int    `yaml:"BLOCK_SIZE"`      // BlockSize defines block size limit.

	// RDO constants
	RoiPerRdo uint64 // RoiPerRdo is the amount of roi corresponding to 1 RDO.

	// Stake config
	ValidatorRegistryLimit       int    `yaml:"VALIDATOR_REGISTRY_LIMIT"`        // ValidatorRegistryLimit defines the maximum count of validators can participate in rdochain.
	ChosenValidatorRewardPercent uint64 `yaml:"CHOSEN_VALIDATOR_REWARD_PERCENT"` // ChosenValidatorRewardPercent defines percent of validator with electors reward
	StakeSlotUnit                uint64 `yaml:"STAKE_SLOT_UNIT"`                 // StakeSlotUnit defines the amount of RDO needed to fill one stake slot.

	GenesisPath     string `yaml:"GENESIS_PATH"`     // GenesisPath defines path to the Genesis JSON file.
	ResponseTimeout int64  `yaml:"RESPONSE_TIMEOUT"` // ResponseTimeout defines timeout for p2p response
	SlotsPerEpoch   uint64 `yaml:"SLOTS_PER_EPOCH"`  // SlotPerEpoch defines slots' number for one epoch

	CommitteeSize int `yaml:"COMMITTEE_SIZE"`

	// NTP pool to check the clock drift
	NTPPool string `yaml:"NTP_POOL"`

	// number of NTP checks to perform
	NTPChecks int `yaml:"NTP_CHECKS"`

	// Specifies what clock drift is considered within boundaries (in ms)
	NTPThreshold int `yaml:"NTP_THRESHOLD"`
}
