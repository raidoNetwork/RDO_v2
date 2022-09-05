package types

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	ssz "github.com/ferranbt/fastssz"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/shared/crypto/secp256k1"
)

type BlockSigner interface {
	// Sign given digest with given key.
	Sign(*BlockHeader, *ecdsa.PrivateKey) (*prototype.Sign, error)

	// SignMixed sign given block with mixed value
	SignMixed(*BlockHeader, []byte, *ecdsa.PrivateKey) (*prototype.Sign, error)

	// Verify sign of given input.
	Verify(*BlockHeader, *prototype.Sign) error

	// VerifyMixed verify sign of given block with mixed value
	VerifyMixed(*BlockHeader, []byte, *prototype.Sign) error
}

type KeccakBlockSigner struct {}

func (kbs *KeccakBlockSigner) Sign(header *BlockHeader, key *ecdsa.PrivateKey) (*prototype.Sign, error){
	return kbs.signRaw(kbs.getBlockDomain(header, nil), key)
}

func (kbs *KeccakBlockSigner) SignMixed(header *BlockHeader, mix []byte, key *ecdsa.PrivateKey) (*prototype.Sign, error){
	hash := kbs.getBlockDomain(header, mix)
	return kbs.signRaw(hash, key)
}

func (kbs *KeccakBlockSigner) signRaw(hash []byte, key *ecdsa.PrivateKey) (*prototype.Sign, error) {
	kdst := crypto.FromECDSA(key)
	signature, err := secp256k1.Sign(hash, kdst)
	if err != nil {
		return nil, err
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)
	sign := &prototype.Sign{
		Address: addr,
		Signature: signature,
	}

	return sign, nil
}

func (kbs *KeccakBlockSigner) Verify(header *BlockHeader, sign *prototype.Sign) error {
	return kbs.verifyRaw(kbs.getBlockDomain(header, nil), sign)
}

func (kbs *KeccakBlockSigner) VerifyMixed(header *BlockHeader, mix []byte, sign *prototype.Sign) error {
	dgst := kbs.getBlockDomain(header, mix)
	return kbs.verifyRaw(dgst, sign)
}

func (kbs *KeccakBlockSigner) verifyRaw(dgst []byte, sign *prototype.Sign) error {
	pubKey, err := crypto.SigToPub(dgst, sign.Signature)
	if err != nil {
		return err
	}

	recAddr := crypto.PubkeyToAddress(*pubKey)
	if !bytes.Equal(recAddr.Bytes(), sign.Address) {
		return errors.New("Wrong signature given!!!")
	}

	return nil
}

func (kbs *KeccakBlockSigner) getBlockDomain(header *BlockHeader, mix []byte) []byte {
	var buf = make([]byte, 0, 75)

	// Put Num
	buf = ssz.MarshalUint64(buf, header.Num)
	buf = ssz.MarshalUint64(buf, header.Slot)

	buf = append(buf, header.Parent...)
	buf = append(buf, header.Version...)
	buf = append(buf, header.TxRoot...)

	if len(mix) > 0 {
		buf = append(buf, mix...)
	}

	return crypto.Keccak256Hash(buf)
}

func MakeBlockSigner(signType string) BlockSigner {
	switch signType {
	case "keccak256":
		return &KeccakBlockSigner{}
	default:
		return nil
	}
}

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

type KeccakTxSigner struct {}

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

	return sign, nil
}

func (s *KeccakTxSigner) Verify(tx *prototype.Transaction) error {
	if len(tx.Signature) != crypto.SignatureLength {
		return errors.New("Wrong signature size.")
	}

	dgst := s.GetTxDomain(tx)
	sign := tx.Signature

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
	return genSalt(crypto.Keccak256(tx.Hash))
}

func genSalt(hash []byte) []byte {
	msg := fmt.Sprintf("\x15RaidoSignedData\n%s", string(hash))
	return crypto.Keccak256([]byte(msg))
}