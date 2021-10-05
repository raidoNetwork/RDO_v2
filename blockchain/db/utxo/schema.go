package utxo

const (
	utxoTable = "unspent_tx_outputs"
	addrToSpentIndex = "addr_to_spent"

	utxoSchema = `
CREATE TABLE IF NOT EXISTS "` + utxoTable + `" (
  "id" INTEGER PRIMARY KEY,
  "tx_type" INTEGER DEFAULT NULL,
  "blockId" INTEGER DEFAULT NULL,
  "hash" TEXT DEFAULT NULL,
  "tx_index" INTEGER DEFAULT NULL,
  "id_index" TEXT DEFAULT NULL,
  "address_from" TEXT DEFAULT NULL,
  "address_to" TEXT DEFAULT NULL,
  "address_node" TEXT DEFAULT NULL,
  "amount" INTEGER DEFAULT NULL,
  "spent" INTEGER DEFAULT NULL,
  "timestamp" INTEGER DEFAULT NULL
);

 CREATE INDEX IF NOT EXISTS "` + addrToSpentIndex + `" ON "` + utxoTable + `"(address_to, spent)
`
)
