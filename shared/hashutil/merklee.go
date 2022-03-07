package hashutil

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
)

var (
	ErrMerkleeTreeCreation = errors.New("Error creating MerkleeTree")
	EmptyTxRoot            = crypto.Keccak256([]byte("empty-tx-body"))
)

// MerkleeRoot return root hash with given []byte
func MerkleeRoot(data [][]byte) (res []byte) {
	size := len(data)

	lvlKoef := size % 2
	lvlCount := size/2 + lvlKoef
	lvlSize := size

	if lvlCount == 0 {
		return EmptyTxRoot
	}

	prevLvl := data
	tree := make([][][]byte, lvlCount)

	var mix []byte
	for i := 0; i < lvlCount; i++ {
		tree[i] = make([][]byte, 0)

		for j := 0; j < lvlSize; j += 2 {
			next := j + 1

			if next == lvlSize {
				mix = prevLvl[j]
			} else {
				mix = prevLvl[j]
				mix = append(mix, prevLvl[next]...)
			}

			mix = crypto.Keccak256(mix)

			tree[i] = append(tree[i], mix)
		}

		lvlSize = len(tree[i])
		prevLvl = tree[i]
	}

	d := tree[len(tree)-1]

	if len(d) > 1 {
		panic(ErrMerkleeTreeCreation)
	}

	res = d[0]

	return res
}

func GenTxRoot(txarr []*prototype.Transaction) []byte {
	data := make([][]byte, 0, len(txarr))

	for _, tx := range txarr {
		data = append(data, tx.Hash)
	}

	root := MerkleeRoot(data)

	return root
}