package types

import (
	"crypto/ecdsa"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
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
