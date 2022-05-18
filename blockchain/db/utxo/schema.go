package utxo

import (
	"github.com/raidoNetwork/RDO_v2/blockchain/db/utxo/dbshared"
)

const (
	utxoSchema = `
CREATE TABLE IF NOT EXISTS ` + "`" + dbshared.UtxoTable + "`" + `(
  ` + "`" + `id` + "`" + ` BIGINT UNSIGNED AUTO_INCREMENT,
  ` + "`" + `tx_type` + "`" + ` INT DEFAULT NULL,
  ` + "`" + `block_id` + "`" + ` BIGINT UNSIGNED DEFAULT NULL,
  ` + "`" + `hash` + "`" + ` VARCHAR(66) DEFAULT NULL,
  ` + "`" + `tx_index` + "`" + ` INT DEFAULT NULL,
  ` + "`" + `address_from` + "`" + ` VARCHAR(44) DEFAULT NULL,
  ` + "`" + `address_to` + "`" + ` VARCHAR(44) DEFAULT NULL,
  ` + "`" + `address_node` + "`" + ` VARCHAR(44) DEFAULT NULL,
  ` + "`" + `amount` + "`" + ` DECIMAL(28, 0) DEFAULT NULL,
  ` + "`" + `timestamp` + "`" + ` BIGINT DEFAULT NULL,
   PRIMARY KEY (` + "`" + `id` + "`" + `),
   KEY ` + dbshared.AddrToNode + ` (address_to, address_node),
   UNIQUE KEY ` + dbshared.HashToTxIndex + ` (hash, tx_index),
   KEY ` + dbshared.BlockIdIndex + `(block_id),
   KEY ` + dbshared.TxTypeToNodeIndex + ` (tx_type, address_node)
);
`
)
