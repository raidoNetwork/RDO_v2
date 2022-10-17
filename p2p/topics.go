package p2p

const (
	mainPrefix 		  = "/raido/"
	blockSuffix 	  = "block-forge"
	txSuffix          = "tx"
	attestationSuffix = "attestation"
	proposalSuffix    = "proposal"
	blockRangeSuffix  = "block-range"
	metaSuffix		  = "metadata"

	MetaProtocol = mainPrefix + metaSuffix
	BlockTopic         = mainPrefix + blockSuffix
	BlockRangeProtocol = mainPrefix + blockRangeSuffix
	TxTopic            = mainPrefix + txSuffix
	AttestationTopic = mainPrefix + attestationSuffix
	ProposalTopic = mainPrefix + proposalSuffix
)

var topicMap = map[string]int{
	BlockTopic: 1,
	TxTopic: 2,
}

var validatorMap = map[string]int{
	AttestationTopic: 3,
	ProposalTopic: 4,
}

