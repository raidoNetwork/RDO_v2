package sync

import (
	"context"
	"github.com/libp2p/go-libp2p-core/peer"
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
	responseCh chan blockResponse
	peers []peer.ID
	data map[uint64]blockResponse
	mu sync.Mutex

	canReadResponse chan struct{}
	requestCh chan blockRequest
}

func NewFetcher(pctx context.Context, from, to uint64, s *Service, peers []peer.ID) *fetcher {
	ctx, cancel := context.WithCancel(pctx)

	f := &fetcher{
		start:           from,
		end:             to,
		s:               s,
		ctx:             ctx,
		cancel:          cancel,
		errCh:           make(chan error),
		responseCh:      make(chan blockResponse),
		peers:           peers,
		canReadResponse: make(chan struct{}),
		data: 			 map[uint64]blockResponse{},
		mu:				 sync.Mutex{},
		requestCh: 		 make(chan blockRequest),
	}

	go f.mainLoop()

	return f
}

func (f *fetcher) mainLoop() {
	wg := &sync.WaitGroup{}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	defer func() {
		close(f.requestCh)
		close(f.responseCh)
	} ()

	go f.handleResponse()

	handlers := (f.end - f.start) / blocksPerRequest
	if handlers > maxRequestHandlers {
		handlers = maxRequestHandlers
	}

	for i := uint64(0); i < handlers; i++ {
		wg.Add(1)
		go f.requestHandlers(wg)
	}

	for start := f.start; start < f.end; start += blocksPerRequest {
		select {
		case err := <-f.errCh:
			log.Errorf("Error with block request %s", err)
			return
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

	blocks, err := f.s.requestBlocks(request, f.peers)
	if err != nil {
		f.errCh <- err
		return
	}

	f.mu.Lock()
	f.data[start] = blockResponse{
		blocks: blocks,
		start: start,
		end: end,
	}
	f.mu.Unlock()
}

func (f *fetcher) Response() <-chan blockResponse {
	return f.responseCh
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
		}

		log.Debugf("Wait for block batch from #%d %d", targetSlot, len(f.data))
		<-time.After(500 * time.Millisecond)
	}
}