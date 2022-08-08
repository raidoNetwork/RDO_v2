package utxo

import (
	"fmt"
	"github.com/raidoNetwork/RDO_v2/blockchain/db/utxo/dbshared"
)

var utxoSchema = fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (" +
	"`id` BIGINT UNSIGNED AUTO_INCREMENT," +
	"`tx_type` INT DEFAULT NULL," +
	"`block_id` BIGINT UNSIGNED DEFAULT NULL," +
	"`hash` VARCHAR(66) DEFAULT NULL," +
	"`tx_index` INT DEFAULT NULL," +
	"`address_from` VARCHAR(44) DEFAULT NULL," +
	"`address_to` VARCHAR(44) DEFAULT NULL," +
	"`address_node` VARCHAR(44) DEFAULT NULL," +
	"`amount` DECIMAL(28, 0) DEFAULT NULL," +
	"`timestamp` BIGINT DEFAULT NULL," +
	`PRIMARY KEY (id),
   KEY %s (address_to, address_node),
   UNIQUE KEY %s (hash, tx_index),
   KEY %s (block_id),
   KEY %s (tx_type, address_node)
);`, dbshared.UtxoTable, dbshared.AddrToNode, dbshared.HashToTxIndex, dbshared.BlockIdIndex, dbshared.TxTypeToNodeIndex)
