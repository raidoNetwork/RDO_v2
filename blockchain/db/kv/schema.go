package kv

var (
	blocksBucket      = []byte("blocks")
	blocksHashBucket  = []byte("blocks-hash")
	blocksNumBucket   = []byte("blocks-num")
	blocksSlotBucket  = []byte("blocks-slot")
	transactionBucket = []byte("transaction")
	addressBucket     = []byte("address-state")

	lastBlockKey      = []byte("last-block")
	genesisBlockKey   = []byte("genesis-block")
	blockNumPrefix    = []byte("block-num")
	blockHashPrefix   = []byte("block-hash")
	blockSlotPrefix   = []byte("block-slot")
	blockPrefix       = []byte("block")
	transactionPrefix = []byte("tx-hash")
	addressPrefix     = []byte("address")

	statsKey = []byte("statistic")
)
