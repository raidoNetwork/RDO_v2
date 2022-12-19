package sync

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

func (s *Service) forkWatcher() {
	ctx, cancel := context.WithCancel(s.ctx)
	bq := &blockQueue{
		queue: make([]*prototype.Block, 0, 100),
		hashMap: map[string]struct{}{},
		numMap: map[uint64]int{},
		ctx: ctx,
		cancel: cancel,
	}

	s.bq = bq

	go s.bq.listen(s.forkBlockEvent)
}

func (s *Service) freeQueue() []*prototype.Block {
	if s.bq == nil {
		return nil
	}

	return s.bq.free()
}