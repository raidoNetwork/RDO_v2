package types

import (
	"bytes"
	"crypto/ecdsa"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/crypto/secp256k1"
)

type TxInputSigner interface {
	// Sign given digest with given key.
	Sign([]byte, *ecdsa.PrivateKey) ([]byte, error)

	// Verify sign of given input.
	Verify(*prototype.TxInput) error
}

func MakeInputSigner(signType string) TxInputSigner {
	switch signType {
	case "keccak256":
		return &KeccakInputSigner{}
	default:
		return nil
	}
}

type KeccakInputSigner struct {
	TxInputSigner
}

func (s *KeccakInputSigner) Sign(dgst []byte, key *ecdsa.PrivateKey) ([]byte, error) {
	if key == nil {
		return nil, errors.New("Empty private key.")
	}

	kdst := crypto.FromECDSA(key)
	sign, err := secp256k1.Sign(dgst, kdst)
	if err != nil {
		return nil, err
	}

	sign[64] += 27

	return sign, nil
}

func (s *KeccakInputSigner) Verify(input *prototype.TxInput) error {
	dgst := GetInputDomain(input.Index, input.Amount, input.Hash)
	sign := input.Signature

	sign[64] -= 27

	pubKey, err := crypto.SigToPub(dgst, sign)
	if err != nil {
		return err
	}

	addr := crypto.PubkeyToAddress(*pubKey)
	if !bytes.Equal(addr.Bytes(), input.Address) {
		return errors.New("Wrong signature given!!!")
	}

	return nil
}

type TxSigner interface {
	// Sign given digest with given key.
	Sign([]byte, *ecdsa.PrivateKey) ([]byte, error)

	// Verify sign of given input.
	Verify(transaction *prototype.Transaction) error
}

func MakeTxSigner(signType string) TxSigner {
	switch signType {
	case "keccak256":
		return &KeccakTxSigner{}
	default:
		return nil
	}
}

type KeccakTxSigner struct {
	TxSigner
}

func (s *KeccakTxSigner) Sign(dgst []byte, key *ecdsa.PrivateKey) ([]byte, error) {
	if key == nil {
		return nil, errors.New("Empty private key.")
	}

	kdst := crypto.FromECDSA(key)
	sign, err := secp256k1.Sign(dgst, kdst)
	if err != nil {
		return nil, err
	}

	sign[64] += 27

	return sign, nil
}

func (s *KeccakTxSigner) Verify(tx *prototype.Transaction) error {
	dgst := GetTxDomain(tx.Hash)
	sign := tx.Signature

	sign[64] -= 27

	pubKey, err := crypto.SigToPub(dgst, sign)
	if err != nil {
		return err
	}

	addr := crypto.PubkeyToAddress(*pubKey)
	if !bytes.Equal(addr.Bytes(), tx.Inputs[0].Address) {
		return errors.New("Wrong signature given!!!")
	}

	return nil
}

func GetTxDomain(hash []byte) []byte {
	return crypto.Keccak256(hash)
}
