package kv

var (
	blocksBucket     = []byte("blocks")
	blocksHashBucket = []byte("blocks-hash")
	blocksNumBucket  = []byte("blocks-num")

	lastBlockKey    = []byte("last-block")
	genesisBlockKey = []byte("genesis-block")
	blockNumPrefix  = []byte("block-num")
	blockHashPrefix = []byte("block-hash")
	blockPrefix     = []byte("block")
)

