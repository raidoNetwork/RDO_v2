package p2p

const (
	mainPrefix = "/raido/"
	blockSuffix = "block-forge"
	txSuffix          = "tx"
	attestationSuffix = "attestationSuffix"
	slashingSuffix          = "slashing"

	BlockTopic = mainPrefix + blockSuffix
	TxTopic = mainPrefix + txSuffix
	AttestationTopic = mainPrefix + attestationSuffix
	SlashTopic = mainPrefix + slashingSuffix
)

var topicMap = map[string]int{
	BlockTopic: 1,
	TxTopic: 2,
}

