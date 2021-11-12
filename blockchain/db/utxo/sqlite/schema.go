package sqlite

import "github.com/raidoNetwork/RDO_v2/blockchain/db/utxo/dbshared"

const (
	utxoSchema = `
CREATE TABLE IF NOT EXISTS "` + dbshared.UtxoTable + `" (
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

 CREATE INDEX IF NOT EXISTS "` + dbshared.AddrToSpentIndex + `" ON "` + dbshared.UtxoTable + `"(address_to, spent);
 CREATE INDEX IF NOT EXISTS "` + dbshared.HashToTxIndex + `" ON "` + dbshared.UtxoTable + `"(hash, tx_index);
 CREATE INDEX IF NOT EXISTS "` + dbshared.BlockIdIndex + `" ON "` + dbshared.UtxoTable + `"(blockId);
`
)
