package rdochain

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/types"
	"time"
)

const startAmount uint64 = 1e12 //10000000000000 // 1 * 10e12

var (
	// genesis addresses for first test
	genesisAddressesBaikals = []string{
		"0x4f4f08987a14e4d91ae2ed0e8bb90642164f71f2",
		"0x9e45bd262effac338beae4edbf4416a60ae21b5d",
		"0x5c7ab002be5d40fef3995d608ff44adf2a783334",
		"0xce3f35411019f0924c44d9bfada15e24106a5781",
		"0x9e020d926983436aae2d6f925cc6980f7bfa6f4e",
		"0xc5bda537525c192d62130d05116e5f5467ba5fa1",
		"0x6e5164defb0313b3bacbfddb249b872c9678713b",
		"0x347d060878601b9261aeb606395b069ccea04347",
		"0xdfd51cab6a9129a2e60bc4102d346459bb7947f0",
		"0x0680957a335d0f7b7b20263851625ebc2169045c",
	}
)

// AccountNum helps OutputManager check balance correctly
var AccountNum uint64 = 10

func (bc *BlockChain) insertGenesis() (*prototype.Block, error) {
	block := bc.getGenesis()
	if block == nil {
		log.Error("Null Genesis.")
		return nil, errors.New("Error creating Genesis.")
	}

	err := bc.db.WriteBlockWithNumKey(block)
	if err != nil {
		log.Errorf("Error saving Genesis block: %s", err)
		return nil, err
	}

	log.Warn("Genesis block successfully saved.")

	return block, nil
}

// getGenesis return GenesisBlock
func (bc *BlockChain) getGenesis() *prototype.Block {
	timestamp := time.Now().UnixNano()

	addresses := genesisAddressesBaikals
	size := len(addresses)

	opts := types.TxOptions{
		Type:    common.GenesisTxType,
		Num:     GenesisBlockNum,
		Inputs:  make([]*prototype.TxInput, 0),
		Outputs: make([]*prototype.TxOutput, 0, size),
		Fee:     0,
		Data:    make([]byte, 0),
	}

	var addr common.Address
	for i := 0; i < size; i++ {
		addr = common.HexToAddress(addresses[i])
		opts.Outputs = append(opts.Outputs, types.NewOutput(addr.Bytes(), startAmount, nil))
	}

	tx, err := types.NewTx(opts, nil)
	if err != nil {
		log.Errorf("You have no genesis. Error: %s", err)
		return nil
	}

	txArr := []*prototype.Transaction{tx}

	// create tx merklee tree root
	txRoot, err := bc.GenTxRoot(txArr)
	if err != nil {
		log.Errorf("getGenesis: Can't find tx root. %s", err)
		return nil
	}

	block := &prototype.Block{
		Num:          GenesisBlockNum,
		Version:      []byte{1, 0, 0},
		Hash:         GenesisHash,
		Parent:       crypto.Keccak256([]byte{}),
		Txroot:       txRoot,
		Timestamp:    uint64(timestamp),
		Transactions: txArr,
		Proposer: &prototype.Sign{
			Address:   crypto.Keccak256([]byte{}),
			Signature: make([]byte, 65),
		},
		Approvers: make([]*prototype.Sign, 0),
		Slashers:  make([]*prototype.Sign, 0),
	}

	return block
}
