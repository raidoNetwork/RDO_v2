package types

import (
	"crypto/ecdsa"
	ssz "github.com/ferranbt/fastssz"
	. "rdo_draft/proto/prototype"
	"rdo_draft/shared/crypto"
)

func NewOutput(address []byte, amount uint64) *TxOutput {
	out := TxOutput{
		Address: address,
		Amount:  amount,
	}

	return &out
}

// NewInput Create new transaction input with output link to the transaction (hash and index)
// and private key for signing.
func NewInput(hash []byte, index uint32, out *TxOutput, key *ecdsa.PrivateKey) (*TxInput, error) {
	input := TxInput{
		Hash:    hash,
		Index:   index,
		Amount:  out.Amount,
		Address: out.Address,
	}

	signer := MakeInputSigner("keccak256")

	// signature digest = Keccak256(index + amount + hash)
	dgst := GetInputDomain(index, out.Amount, hash)
	sign, err := signer.Sign(dgst, key)
	if err != nil {
		return nil, err
	}

	signa := make([]byte, 31)
	signa = append(signa, sign...)

	input.Sign = &Sign{
		Signature: signa,
	}

	return &input, nil
}

func GetInputDomain(index uint32, amount uint64, hash []byte) []byte {
	domain := make([]byte, 0)

	domain = ssz.MarshalUint32(domain, index)
	domain = ssz.MarshalUint64(domain, amount)
	domain = append(domain, hash...)

	domain = crypto.Keccak256Hash(domain)

	return domain
}
