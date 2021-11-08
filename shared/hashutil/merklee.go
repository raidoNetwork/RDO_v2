package hashutil

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/crypto"
)

var (
	ErrMerkleeTreeCreation = errors.New("Error creating MerkleeTree")
)

// merkleeRoot return root hash with given []byte
func MerkleeRoot(data [][]byte) (res []byte, err error) {
	size := len(data)

	lvlKoef := size % 2
	lvlCount := size/2 + lvlKoef
	lvlSize := size

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
		return nil, ErrMerkleeTreeCreation
	}

	res = d[0]

	return res, nil
}
