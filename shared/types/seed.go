package types

import (
	"crypto/rand"
	"math/big"

	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

// This package is used for generating random seeds so validators
// can agree on a single random number for determining the proposer
func NewSeed(key *keystore.ValidatorAccount) (*prototype.Seed, error) {
	big, err := rand.Int(rand.Reader, big.NewInt(1<<32))
	if err != nil {
		return nil, err
	}
	intseed := uint32(big.Int64())
	seed := &prototype.Seed{Seed: intseed, Proposer: &prototype.Sign{}}
	signer := GetSeedSigner()

	signature, err := signer.Sign(seed, key.Key())
	if err != nil {
		return nil, err
	}

	address := key.Addr().Bytes()

	seed.Proposer.Address = address
	seed.Proposer.Signature = signature
	return seed, nil
}

var seedSigner SeedSigner

func GetSeedSigner() SeedSigner {
	// create block signer if not exist
	if seedSigner == nil {
		seedSigner = MakeSeedSigner("keccak256")
	}

	return seedSigner
}
