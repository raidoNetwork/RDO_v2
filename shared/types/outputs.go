package types

import (
	"crypto/ecdsa"
	ssz "github.com/ferranbt/fastssz"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
)


func NewOutput(address []byte, amount uint64, node []byte) *prototype.TxOutput {
	if address == nil {
		address = make([]byte, common.AddressLength)
	}

	out := prototype.TxOutput{
		Address: address,
		Amount:  amount,
		Node:    node,
	}

	return &out
}

// NewInput Create new transaction input with output link to the transaction (hash and index)
// and private key for signing.
func NewInput(hash []byte, index uint32, out *prototype.TxOutput, key *ecdsa.PrivateKey) (*prototype.TxInput, error) {
	input := prototype.TxInput{
		Hash:    hash,
		Index:   index,
		Amount:  out.Amount,
		Address: out.Address,
	}

	if key != nil {
		signer := MakeInputSigner("keccak256")

		// signature digest = Keccak256(index + amount + hash)
		dgst := GetInputDomain(index, out.Amount, hash)
		sign, err := signer.Sign(dgst, key)
		if err != nil {
			return nil, err
		}

		input.Signature = sign
	} else {
		input.Signature = make([]byte, 65)
	}

	return &input, nil
}

func GetInputDomain(index uint32, amount uint64, hash []byte) []byte {
	domain := make([]byte, 0)

	domain = ssz.MarshalUint32(domain, index)
	domain = ssz.MarshalUint64(domain, amount)
	domain = append(domain, hash...)

	domain = crypto.Keccak256(domain)

	return domain
}
