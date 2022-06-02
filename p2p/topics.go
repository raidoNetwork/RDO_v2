package p2p

const (
	mainPrefix 		  = "/raido/"
	blockSuffix 	  = "block-forge"
	txSuffix          = "tx"
	attestationSuffix = "attestation-suffix"
	slashingSuffix    = "slashing"
	blockRangeSuffix  = "block-range"

	BlockTopic = mainPrefix + blockSuffix
	BlockRangeTopic = mainPrefix + blockRangeSuffix
	TxTopic = mainPrefix + txSuffix
	AttestationTopic = mainPrefix + attestationSuffix
	SlashTopic = mainPrefix + slashingSuffix
)

var topicMap = map[string]int{
	BlockTopic: 1,
	TxTopic: 2,
}

