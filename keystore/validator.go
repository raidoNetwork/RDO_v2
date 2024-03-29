package keystore

import (
	"crypto/ecdsa"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
	"github.com/raidoNetwork/RDO_v2/utils/file"
)

type ValidatorAccount struct {
	key *ecdsa.PrivateKey
	addr common.Address
}

func (v *ValidatorAccount) Key() *ecdsa.PrivateKey {
	return v.key
}

func (v *ValidatorAccount) Addr() common.Address {
	return v.addr
}

func NewValidatorAccount(key *ecdsa.PrivateKey) *ValidatorAccount {
	addr := crypto.PubkeyToAddress(key.PublicKey)
	return &ValidatorAccount{key, addr}
}

func NewValidatorAccountFromFile(path string) (*ValidatorAccount, error) {
	if !file.FileExists(path) {
		return &ValidatorAccount{}, nil
	}

	key, err := crypto.LoadECDSA(path)
	if err != nil {
		log.Error("Error loading validator: ", err)
		return nil, err
	}

	return NewValidatorAccount(key), nil
}
