package mysql

import (
	"github.com/raidoNetwork/RDO_v2/blockchain/db/utxo/dbshared"
)

const (
	utxoSchema = `
CREATE TABLE IF NOT EXISTS ` + "`" + dbshared.UtxoTable + "`" + `(
  ` + "`" + `id` + "`" + ` BIGINT UNSIGNED AUTO_INCREMENT,
  ` + "`" + `tx_type` + "`" + ` INT DEFAULT NULL,
  ` + "`" + `blockId` + "`" + ` BIGINT UNSIGNED DEFAULT NULL,
  ` + "`" + `hash` + "`" + ` VARCHAR(66) DEFAULT NULL,
  ` + "`" + `tx_index` + "`" + ` INT DEFAULT NULL,
  ` + "`" + `id_index` + "`" + ` INT DEFAULT NULL,
  ` + "`" + `address_from` + "`" + ` VARCHAR(44) DEFAULT NULL,
  ` + "`" + `address_to` + "`" + ` VARCHAR(44) DEFAULT NULL,
  ` + "`" + `address_node` + "`" + ` VARCHAR(66) DEFAULT NULL,
  ` + "`" + `amount` + "`" + ` BIGINT UNSIGNED DEFAULT NULL,
  ` + "`" + `spent` + "`" + ` TINYINT DEFAULT NULL,
  ` + "`" + `timestamp` + "`" + ` BIGINT DEFAULT NULL,
   PRIMARY KEY (` + "`" + `id` + "`" + `),
   KEY ` + dbshared.AddrToSpentIndex + ` (address_to, spent),
   KEY ` + dbshared.HashToTxIndex + ` (hash, tx_index),
   KEY ` + dbshared.BlockIdIndex + `(blockId)
);
`
)
