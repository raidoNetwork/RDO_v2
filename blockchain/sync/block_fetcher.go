package sync

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
)

const (
	maxRequestHandlers     = 10
	responseHandlerTimeout = 200 * time.Millisecond
	aheadBlocksCount       = 10
	dataBufferLength       = 10
)

type blockResponse struct {
	blocks     []*prototype.Block
	peer       peer.ID
	start, end uint64
}

type blockRequest struct {
	start, end uint64
}

type fetcher struct {
	start, end uint64
	s          *Service
	ctx        context.Context
	cancel     context.CancelFunc

	dataBuffer chan struct{}
	errCh      chan error
	requestCh  chan blockRequest
	responseCh chan *blockResponse
	data       map[uint64]*blockResponse

	mu           sync.Mutex
	maxBlockMode bool
}

func NewFetcher(pctx context.Context, from, to uint64, s *Service, maxBlockMode bool) *fetcher {
	ctx, cancel := context.WithCancel(pctx)

	f := &fetcher{
		start:        from,
		end:          to,
		s:            s,
		ctx:          ctx,
		cancel:       cancel,
		errCh:        make(chan error),
		responseCh:   make(chan *blockResponse),
		dataBuffer:   make(chan struct{}, dataBufferLength),
		data:         map[uint64]*blockResponse{},
		mu:           sync.Mutex{},
		requestCh:    make(chan blockRequest),
		maxBlockMode: maxBlockMode,
	}

	go f.mainLoop()

	return f
}

func (f *fetcher) mainLoop() {
	wg := &sync.WaitGroup{}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	// start response handler
	go f.handleResponse()

	// fill the dataBuffer channel
	for i := 0; i < 10; i++ {
		f.dataBuffer <- struct{}{}
	}

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
				end:   end,
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
	// wait for the sync
	<-f.dataBuffer
	request := &prototype.BlockRequest{
		StartSlot: start,
		Count:     end - start + aheadBlocksCount,
		Step:      1,
	}

	if start == 0 {
		request.Count += 1
	}

	peers, _ := f.s.findPeers(f.maxBlockMode)
	blocks, peer, err := f.s.requestBlocks(request, peers)
	if err != nil {
		f.errCh <- err
		return
	}

	f.mu.Lock()
	f.data[start] = &blockResponse{
		blocks: blocks,
		peer:   peer,
		start:  start,
		end:    end,
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

		f.mu.Lock()
		log.Infof("Wait for block batch from slot #%d. Blocks stored: %d", targetSlot, len(f.data)*blocksPerRequest)
		time.Sleep(200 * time.Millisecond)
		f.mu.Unlock()
		// add some kind of sync
	}

	close(f.responseCh)
}
