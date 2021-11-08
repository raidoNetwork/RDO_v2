package types

import (
	"crypto/ecdsa"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/crypto/secp256k1"
)

type TxInputSigner interface {
	Sign([]byte, *ecdsa.PrivateKey) ([]byte, error)
	Verify([]byte, []byte, []byte) bool
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
	return crypto.Sign(dgst, key)
}

func (s *KeccakInputSigner) Verify(dgst []byte, sign []byte, key []byte) bool {
	return crypto.VerifySignature(key, dgst, sign)
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

	//pubKey, err := crypto.SigToPub(dgst, sign)
	pubKey, err := secp256k1.RecoverPubkey(dgst, sign)
	if err != nil {
		return err
	}

	ecdsaPubKey, err := crypto.UnmarshalPubkey(pubKey)
	if err != nil {
		return err
	}

	// get users address
	usrAddr := common.BytesToAddress(tx.Inputs[0].Address)

	addr := crypto.PubkeyToAddress(*ecdsaPubKey)
	if addr.Hex() != usrAddr.Hex() {
		return errors.New("Wrong signature given!!!")
	}

	return nil
}

func GetTxDomain(hash []byte) []byte {
	return crypto.Keccak256(hash)
}