package types

import (
	"math/rand"
	"time"

	"github.com/raidoNetwork/RDO_v2/keystore"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

// This package is used for generating random seeds so validators
// can agree on a single random number for determining the proposer
func NewSeed(key *keystore.ValidatorAccount) (*prototype.Seed, error) {
	genSeed := time.Now().UnixNano()
	rand.Seed(genSeed)
	num := rand.Uint32()
	seed := &prototype.Seed{Seed: num, Proposer: &prototype.Sign{}}
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
