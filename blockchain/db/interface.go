package db

import (
	"github.com/raidoNetwork/RDO_v2/blockchain/db/iface"
)

type Database = iface.Database

type BlockStorage = iface.ChainStorage

type OutputStorage = iface.OutputStorage

type OutputDatabase = iface.OutputDatabase

type SQLConfig = iface.SQLConfig
