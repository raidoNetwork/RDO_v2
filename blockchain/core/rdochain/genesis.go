package rdochain

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"github.com/raidoNetwork/RDO_v2/utils/hash"
	"io/ioutil"
)

var ErrMissedGenesis = errors.New("Unset Genesis JSON path.")

const outputsPerTxLimit = 2000

// insertGenesis inserts Genesis to the database if it is not exists
func (bc *BlockChain) insertGenesis() error {
	block, err := bc.db.GetGenesis()
	if err != nil {
		return err
	}

	// Genesis already in database
	if block != nil {
		bc.genesisBlock = block
		bc.genesisHash = block.Hash
		return nil
	}

	block = bc.createGenesis()
	if block == nil {
		return errors.New("Error creating Genesis.")
	}

	err = bc.db.SaveGenesis(block)
	if err != nil {
		log.Errorf("Error saving Genesis block: %s", err)
		return err
	}

	bc.genesisBlock = block

	log.Warn("Genesis block successfully saved.")

	return nil
}

// castGenesisOutputs
func (bc *BlockChain) castGenesisOutputs(data *types.GenesisBlock) []*prototype.TxOutput {
	size := len(data.Outputs)
	outs := make([]*prototype.TxOutput, 0, size)

	var address common.Address
	for addr, amount := range data.Outputs {
		address = common.HexToAddress(addr)
		outs = append(outs, types.NewOutput(address.Bytes(), amount, nil))
	}

	return outs
}

// createGenesis return GenesisBlock
func (bc *BlockChain) createGenesis() *prototype.Block {
	// try to load Genesis data from JSON file.
	genesisData, err := bc.loadGenesisData()
	if err != nil {
		log.Errorf("Error creating Genesis: %s", err)
		return nil
	}

	genesisOutputs := bc.castGenesisOutputs(genesisData)
	genesisOutputsCount := len(genesisOutputs)
	log.Debugf("Create Genesis from JSON. Outputs: %d", genesisOutputsCount)
	txCount := genesisOutputsCount / outputsPerTxLimit
	left := genesisOutputsCount % outputsPerTxLimit

	if left != 0 {
		txCount++
	}

	txArr := make([]*prototype.Transaction, 0)
	for i := 0; i < txCount; i++ {
		start := i * outputsPerTxLimit
		end := start + outputsPerTxLimit

		if i == txCount - 1 && left != 0 {
			end = start + left
		}

		opts := types.TxOptions{
			Type:    common.GenesisTxType,
			Num:     GenesisBlockNum,
			Inputs:  make([]*prototype.TxInput, 0),
			Outputs: genesisOutputs[start:end],
			Fee:     0,
			Data:    make([]byte, 0),
		}
		tx, err := types.NewPbTransaction(opts, nil)
		if err != nil {
			log.Errorf("You have no genesis. Error: %s", err)
			return nil
		}

		txArr = append(txArr, tx)
	}

	// create tx merklee tree root
	txRoot := hash.GenTxRoot(txArr)
	block := &prototype.Block{
		Num:          GenesisBlockNum,
		Slot:		  0,
		Version:      []byte{1, 0, 0},
		Hash:         bc.genesisHash,
		Parent:       crypto.Keccak256([]byte{}),
		Txroot:       txRoot,
		Timestamp:    genesisData.Timestamp,
		Transactions: txArr,
		Proposer: &prototype.Sign{
			Address:   common.HexToAddress(common.BlackHoleAddress).Bytes(),
			Signature: make([]byte, crypto.SignatureLength),
		},
	}

	return block
}

// loadGenesisData
func (bc *BlockChain) loadGenesisData() (*types.GenesisBlock, error) {
	if bc.cfg.GenesisPath == "" {
		return nil, ErrMissedGenesis
	}

	genesisData := new(types.GenesisBlock)

	buf, err := ioutil.ReadFile(bc.cfg.GenesisPath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(buf, genesisData)
	if err != nil {
		return nil, err
	}

	bc.genesisHash = common.HexToHash(genesisData.Hash)

	return genesisData, nil
}

// GetGenesis returns Genesis block stored in memory
func (bc *BlockChain) GetGenesis() *prototype.Block {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	return bc.genesisBlock
}
