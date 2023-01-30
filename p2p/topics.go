package p2p

const (
	mainPrefix        = "/raido/"
	blockSuffix       = "block-forge"
	txSuffix          = "tx"
	seedSuffix        = "seed"
	attestationSuffix = "attestation"
	proposalSuffix    = "proposal"
	blockRangeSuffix  = "block-range"
	metaSuffix        = "metadata"

	MetaProtocol       = mainPrefix + metaSuffix
	BlockTopic         = mainPrefix + blockSuffix
	BlockRangeProtocol = mainPrefix + blockRangeSuffix
	TxTopic            = mainPrefix + txSuffix
	SeedTopic          = mainPrefix + seedSuffix
	AttestationTopic   = mainPrefix + attestationSuffix
	ProposalTopic      = mainPrefix + proposalSuffix
)

var topicMap = map[string]int{
	BlockTopic: 1,
	TxTopic:    2,
}

var validatorMap = map[string]int{
	SeedTopic:        3,
	AttestationTopic: 4,
	ProposalTopic:    5,
}
