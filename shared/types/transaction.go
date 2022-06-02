package types

import (
	"crypto/ecdsa"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/utils/hash"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

var log = logrus.WithField("prefix", "types")

var txSigner TxSigner
var changeOutput = NewOutput(make([]byte, 20), 200, nil)

func getTxSigner() TxSigner {
	// create transaction signer if not exist
	if txSigner == nil {
		txSigner = MakeTxSigner("keccak256")
	}

	return txSigner
}

type TxOptions struct {
	Inputs  []*prototype.TxInput
	Outputs []*prototype.TxOutput
	Fee     uint64
	Data    []byte
	Num     uint64
	Type    uint32
}

// NewTx creates new transaction with given options
func NewTx(opts TxOptions, key *ecdsa.PrivateKey) (*prototype.Transaction, error) {
	tx := new(prototype.Transaction)

	tx.Num = opts.Num
	tx.Type = opts.Type
	tx.Timestamp = uint64(time.Now().UnixNano())
	tx.Fee = opts.Fee
	tx.Data = opts.Data
	tx.Inputs = opts.Inputs
	tx.Outputs = opts.Outputs

	hash, err := hash.TxHash(tx)
	if err != nil {
		log.Errorf("NewTx: Error generating tx hash. Error: %s", err)
		return nil, err
	}

	tx.Hash = hash[:]

	if key != nil {
		err = SignTx(tx, key)
		if err != nil {
			return nil, err
		}
	} else {
		tx.Signature = make([]byte, crypto.SignatureLength)
	}

	return tx, nil
}

func CountTxFee(opts TxOptions) (uint64, uint64) {
	tx := new(prototype.Transaction)

	tx.Num = opts.Num
	tx.Type = opts.Type
	tx.Timestamp = uint64(time.Now().UnixNano())
	tx.Fee = opts.Fee
	tx.Data = opts.Data
	tx.Inputs = opts.Inputs
	tx.Outputs = opts.Outputs
	tx.Hash = make([]byte, 32)
	tx.Signature = make([]byte, crypto.SignatureLength)

	fee := tx.GetRealFee()

	tx.Outputs = append(tx.Outputs, changeOutput)

	extraFee := tx.GetRealFee()

	return fee, extraFee
}

// SignTx create transaction signature with given private key
func SignTx(tx *prototype.Transaction, key *ecdsa.PrivateKey) error {
	sign, err := getTxSigner().Sign(tx, key)
	if err != nil {
		return err
	}

	tx.Signature = sign

	return nil
}

func NewTransaction(tx *prototype.Transaction) *Transaction {
	td := Transaction{
		tx:        tx,
		size:      tx.SizeSSZ(),
		fee:       tx.Fee,
		num:       tx.Num,
		timestamp: tx.Timestamp,
		alias:     make([]string, 0),
	}

	return &td
}

type Transaction struct {
	tx        *prototype.Transaction
	size      int
	fee       uint64
	num       uint64
	alias     []string
	timestamp uint64
	lock      sync.Mutex
}

func (td *Transaction) GetTx() *prototype.Transaction {
	td.lock.Lock()
	defer td.lock.Unlock()

	return td.tx
}

func (td *Transaction) Size() int {
	td.lock.Lock()
	defer td.lock.Unlock()

	return td.size
}

func (td *Transaction) FeePrice() uint64 {
	td.lock.Lock()
	defer td.lock.Unlock()

	return td.fee
}

func (td *Transaction) Num() uint64 {
	td.lock.Lock()
	defer td.lock.Unlock()

	return td.num
}

func (td *Transaction) Timestamp() uint64 {
	td.lock.Lock()
	defer td.lock.Unlock()

	return td.timestamp
}

func (td *Transaction) AddAlias(hash string) {
	td.lock.Lock()
	defer td.lock.Unlock()

	td.alias = append(td.alias, hash)
}

func (td *Transaction) GetAlias() []string {
	td.lock.Lock()
	defer td.lock.Unlock()

	return td.alias
}
