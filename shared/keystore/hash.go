package keystore

import (
	"golang.org/x/crypto/sha3"
	"hash"
	"sync"

	"github.com/minio/sha256-simd"
)

var sha256Pool = sync.Pool{New: func() interface{} {
	return sha256.New()
}}

func Hash(data []byte) [32]byte {
	h, ok := sha256Pool.Get().(hash.Hash)
	if !ok {
		h = sha256.New()
	}
	defer sha256Pool.Put(h)
	h.Reset()

	var b [32]byte

	// The hash interface never returns an error, for that reason
	// we are not handling the error below. For reference, it is
	// stated here https://golang.org/pkg/hash/#Hash

	// #nosec G104
	h.Write(data)
	h.Sum(b[:0])

	return b
}

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		// #nosec G104
		d.Write(b)
	}
	return d.Sum(nil)
}
