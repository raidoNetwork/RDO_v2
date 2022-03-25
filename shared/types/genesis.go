package types

import (
	"encoding/json"
	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"io/ioutil"
	"path/filepath"
	"time"
)

type GenesisBlock struct {
	Outputs   map[string]uint64 `json:"outputs"`   // map hex address -> amount
	Hash      string            `json:"hash"`      // Genesis hash
	Timestamp uint64			`json:"timestamp"` // Timestamp
}

func CreateGenesisJSON(accman *keystore.AccountManager, path string) error {
	file := filepath.Join(path, "genesis.json")

	genesisStruct := new(GenesisBlock)

	balances := map[string]uint64{}
	for addr, _ := range accman.GetPairs() {
		balances[addr] = common.StartAmount
	}

	genesisStruct.Hash = crypto.Keccak256Hash([]byte("genesis-block")).Hex()
	genesisStruct.Outputs = balances
	genesisStruct.Timestamp = uint64(time.Now().UnixNano())

	data, err := json.Marshal(genesisStruct)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(file, data, 0600)
}
