package sync

import (
	"context"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"sync"
	"time"
)

const maxRequestHandlers = 10

type blockResponse struct {
	blocks []*prototype.Block
	start, end uint64
}

type blockRequest struct {
	start, end uint64
}

type fetcher struct {
	start, end uint64
	s *Service
	ctx context.Context
	cancel context.CancelFunc

	errCh chan error
	requestCh chan blockRequest
	responseCh chan *blockResponse
	data map[uint64]*blockResponse

	mu sync.Mutex
	maxBlockMode bool
}


func NewFetcher(pctx context.Context, from, to uint64, s *Service, maxBlockMode bool) *fetcher {
	ctx, cancel := context.WithCancel(pctx)

	f := &fetcher{
		start:           from,
		end:             to,
		s:               s,
		ctx:             ctx,
		cancel:          cancel,
		errCh:           make(chan error),
		responseCh:      make(chan *blockResponse),
		data: 			 map[uint64]*blockResponse{},
		mu:				 sync.Mutex{},
		requestCh: 		 make(chan blockRequest),
		maxBlockMode: 	 maxBlockMode,
	}

	go f.mainLoop()

	return f
}

func (f *fetcher) mainLoop() {
	wg := &sync.WaitGroup{}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	defer func() {
		wg.Wait()
		close(f.responseCh)
	} ()

	// start response handler
	go f.handleResponse()

	// start request workers
	for i := 0; i < maxRequestHandlers; i++ {
		wg.Add(1)
		go f.requestHandlers(wg)
	}

	for start := f.start; start < f.end; start += blocksPerRequest {
		// waiting for the peers
		f.s.waitMinimumPeersForSync(f.maxBlockMode)

		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			end := start + blocksPerRequest
			if end > f.end {
				end = f.end
			}

			f.requestCh <- blockRequest{
				start: start,
				end: end,
			}
		}
	}

	close(f.requestCh)
	wg.Wait()
}

func (f *fetcher) requestHandlers(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case req, ok := <-f.requestCh:
			if !ok {
				return
			}

			f.request(req.start, req.end)
		case <-f.ctx.Done():
			return
		}
	}
}

func (f *fetcher) request(start, end uint64) {
	request := &prototype.BlockRequest{
		StartSlot: start,
		Count:     end - start,
		Step:      1,
	}

	peers, _ := f.s.findPeers(f.maxBlockMode)
	blocks, err := f.s.requestBlocks(request, peers)
	if err != nil {
		f.errCh <- err
		return
	}

	f.mu.Lock()
	_, exists := f.data[start]
	if exists {
		panic("Double block request")
	}

	f.data[start] = &blockResponse{
		blocks: blocks,
		start: start,
		end: end,
	}
	f.mu.Unlock()
}

func (f *fetcher) Response() <-chan *blockResponse {
	return f.responseCh
}

func (f *fetcher) Error() <-chan error {
	return f.errCh
}

func (f *fetcher) handleResponse() {
	targetSlot := f.start

	for targetSlot < f.end {
		f.mu.Lock()
		res, exists := f.data[targetSlot]
		f.mu.Unlock()

		if exists {
			f.responseCh <- res

			f.mu.Lock()
			delete(f.data, targetSlot)
			f.mu.Unlock()

			targetSlot += blocksPerRequest
			continue
		}

		log.Infof("Wait for block batch from slot #%d. Blocks stored: ~%d", targetSlot, len(f.data) * blocksPerRequest)
		<-time.After(200 * time.Millisecond)
	}
}