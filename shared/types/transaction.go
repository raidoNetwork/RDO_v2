package types

import (
	"crypto/ecdsa"
	"sync"
	"time"

	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/utils/hash"
	"github.com/sirupsen/logrus"
)

type ExecutionStatus uint32

const (
	TxSuccess ExecutionStatus = iota + 1
	TxFailed
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
	Inputs    []*prototype.TxInput
	Outputs   []*prototype.TxOutput
	Fee       uint64
	Data      []byte
	Num       uint64
	Type      uint32
	Timestamp uint64
}

// NewPbTransaction creates new transaction with given options
func NewPbTransaction(opts TxOptions, key *ecdsa.PrivateKey) (*prototype.Transaction, error) {
	tx := new(prototype.Transaction)

	tx.Num = opts.Num
	tx.Type = opts.Type
	tx.Fee = opts.Fee
	tx.Data = opts.Data
	tx.Inputs = opts.Inputs
	tx.Outputs = opts.Outputs

	if opts.Timestamp > 0 {
		tx.Timestamp = opts.Timestamp
	} else {
		tx.Timestamp = uint64(time.Now().UnixNano())
	}

	hash, err := hash.TxHash(tx)
	if err != nil {
		log.Errorf("NewPbTransaction: Error generating tx hash. Error: %s", err)
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

func NewTransaction(pbtx *prototype.Transaction) *Transaction {
	if pbtx == nil {
		return nil
	}

	var from common.Address
	if len(pbtx.Inputs) > 0 {
		from = common.BytesToAddress(pbtx.Inputs[0].Address)
	}

	size := pbtx.SizeSSZ()
	return &Transaction{
		tx:        pbtx,
		hash:      common.BytesToHash(pbtx.Hash),
		from:      from,
		txType:    pbtx.Type,
		size:      size,
		feePrice:  pbtx.Fee,
		fee:       pbtx.Fee * uint64(size),
		num:       pbtx.Num,
		timestamp: pbtx.Timestamp,
		doubles:   make([]*Transaction, 0),
		inputs:    inputSlice(pbtx.Inputs),
		outputs:   outputSlice(pbtx.Outputs),
	}
}

type Transaction struct {
	tx        *prototype.Transaction
	hash      common.Hash
	from      common.Address
	txType    uint32
	size      int
	feePrice  uint64
	fee       uint64
	num       uint64
	timestamp uint64
	doubles   []*Transaction
	inputs    []*Input
	outputs   []*Output
	lock      sync.Mutex

	// tx state
	dropped bool
	forged  bool
}

func (tx *Transaction) GetTx() *prototype.Transaction {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	return tx.tx
}

func (tx *Transaction) Size() int {
	return tx.size
}

func (tx *Transaction) FeePrice() uint64 {
	return tx.feePrice
}

func (tx *Transaction) Fee() uint64 {
	return tx.fee
}

func (tx *Transaction) Num() uint64 {
	return tx.num
}

func (tx *Transaction) Timestamp() uint64 {
	return tx.timestamp
}

func (tx *Transaction) Hash() common.Hash {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	return tx.hash
}

func (tx *Transaction) From() common.Address {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	return tx.from
}

func (tx *Transaction) Type() uint32 {
	return tx.txType
}

func (tx *Transaction) Inputs() []*Input {
	return tx.inputs
}

func (tx *Transaction) Outputs() []*Output {
	return tx.outputs
}

func (tx *Transaction) AllSenders() []common.Address {
	res := make([]common.Address, 0, 1)
	seen := make(map[string]struct{})
	for _, in := range tx.inputs {
		addr := in.Address().Hex()
		if _, isAdded := seen[addr]; isAdded {
			continue
		}

		seen[addr] = struct{}{}
		res = append(res, in.Address())
	}
	return res
}

func (tx *Transaction) Drop() {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	tx.dropped = true
}

func (tx *Transaction) IsDropped() bool {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	return tx.dropped
}

func (tx *Transaction) Forge() {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	tx.forged = true
}

func (tx *Transaction) IsForged() bool {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	return tx.forged
}

func (tx *Transaction) DiscardForge() {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	tx.forged = false
}

func (tx *Transaction) SetStatus(status ExecutionStatus) {
	tx.lock.Lock()
	tx.tx.Status = uint32(status)
	tx.lock.Unlock()
}

func (tx *Transaction) Status() ExecutionStatus {
	tx.lock.Lock()
	defer tx.lock.Unlock()

	return ExecutionStatus(tx.tx.Status)
}

type Input struct {
	hash    common.Hash
	address common.Address
	index   uint32
	amount  uint64
	node    common.Address
}

func (in *Input) Hash() common.Hash {
	return in.hash
}

func (in *Input) Address() common.Address {
	return in.address
}

func (in *Input) Index() uint32 {
	return in.index
}

func (in *Input) Amount() uint64 {
	return in.amount
}

func (in *Input) Node() common.Address {
	return in.node
}

func newInput(inpb *prototype.TxInput) *Input {
	return &Input{
		hash:    common.BytesToHash(inpb.Hash),
		address: common.BytesToAddress(inpb.Address),
		index:   inpb.Index,
		amount:  inpb.Amount,
		node:    common.BytesToAddress(inpb.Node),
	}
}

func inputSlice(inpbarr []*prototype.TxInput) []*Input {
	res := make([]*Input, 0, len(inpbarr))
	for _, inpb := range inpbarr {
		res = append(res, newInput(inpb))
	}
	return res
}

type Output struct {
	address common.Address
	node    common.Address
	amount  uint64
}

func (out *Output) Address() common.Address {
	return out.address
}

func (out *Output) Node() common.Address {
	return out.node
}

func (out *Output) Amount() uint64 {
	return out.amount
}

func newOutput(outpb *prototype.TxOutput) *Output {
	return &Output{
		address: common.BytesToAddress(outpb.Address),
		node:    common.BytesToAddress(outpb.Node),
		amount:  outpb.Amount,
	}
}

func outputSlice(outpbarr []*prototype.TxOutput) []*Output {
	res := make([]*Output, 0, len(outpbarr))
	for _, outpb := range outpbarr {
		res = append(res, newOutput(outpb))
	}
	return res
}
