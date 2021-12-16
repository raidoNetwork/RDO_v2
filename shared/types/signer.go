package types

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/crypto/secp256k1"
)

type TxSigner interface {
	// Sign given digest with given key.
	Sign(*prototype.Transaction, *ecdsa.PrivateKey) ([]byte, error)

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
	signSalt []byte
	buf      []byte
}

func (s *KeccakTxSigner) Sign(tx *prototype.Transaction, key *ecdsa.PrivateKey) ([]byte, error) {
	if key == nil {
		return nil, errors.New("Empty private key.")
	}

	dgst := s.GetTxDomain(tx)

	kdst := crypto.FromECDSA(key)
	sign, err := secp256k1.Sign(dgst, kdst)
	if err != nil {
		return nil, err
	}

	sign[64] += 27

	return sign, nil
}

func (s *KeccakTxSigner) Verify(tx *prototype.Transaction) error {
	if len(tx.Signature) != crypto.SignatureLength {
		return errors.New("Wrong signature size.")
	}

	dgst := s.GetTxDomain(tx)
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

func (s *KeccakTxSigner) GetTxDomain(tx *prototype.Transaction) []byte {
	msg := fmt.Sprintf("\x55RaidoSignedData\n32%s", string(crypto.Keccak256(tx.Hash)))
	return crypto.Keccak256([]byte(msg))
}
