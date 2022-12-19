package sync

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"sync"
)

type blockQueue struct {
	queue []*prototype.Block
	hashMap map[string]struct{}
	numMap map[uint64]int

	mu sync.Mutex

	ctx context.Context
	cancel context.CancelFunc
}

func (bq *blockQueue) listen(data <-chan *prototype.Block) {
	for {
		select {
		case b := <-data:
			hash := common.Encode(b.Hash)

			bq.mu.Lock()
			if _, exists := bq.hashMap[hash]; exists {
				bq.mu.Unlock()
				continue
			}

			if _, exists := bq.numMap[b.Num]; exists {
				bq.mu.Unlock()
				// todo add fork choice by num
				continue
			}

			bq.queue = append(bq.queue, b)
			bq.hashMap[hash] = struct{}{}
			bq.numMap[b.Num] = len(bq.queue) - 1
			bq.mu.Unlock()
		case <-bq.ctx.Done():
			return
		}

	}
}

func (bq *blockQueue) free() []*prototype.Block {
	bq.cancel()

	defer func() {
		bq.mu.Lock()
		bq.queue = make([]*prototype.Block, 0, 100)
		bq.hashMap = map[string]struct{}{}
		bq.numMap = map[uint64]int{}
		bq.mu.Unlock()
	} ()

	return bq.queue
}
