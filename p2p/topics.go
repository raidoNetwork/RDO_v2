package p2p

// todo add version to each protocol

const (
	mainPrefix 		  = "/raido/"
	blockSuffix 	  = "block-forge"
	txSuffix          = "tx"
	attestationSuffix = "attestation-suffix"
	slashingSuffix    = "slashing"
	blockRangeSuffix  = "block-range"
	metaSuffix		  = "metadata"

	MetaProtocol = mainPrefix + metaSuffix
	BlockTopic         = mainPrefix + blockSuffix
	BlockRangeProtocol = mainPrefix + blockRangeSuffix
	TxTopic            = mainPrefix + txSuffix
	AttestationTopic = mainPrefix + attestationSuffix
	SlashTopic = mainPrefix + slashingSuffix
)

var topicMap = map[string]int{
	BlockTopic: 1,
	TxTopic: 2,
}

