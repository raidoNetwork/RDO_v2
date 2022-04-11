package p2p

const (
	mainPrefix = "/raido/"
	blockSuffix = "block-forge"
	txSuffix = "tx"

	BlockTopic = mainPrefix + blockSuffix
	TxTopic = mainPrefix + txSuffix
)

var topicMap = map[string]int{
	BlockTopic: 1,
	TxTopic: 2,
}

