package sync

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"sort"
	"time"
)

func (s *Service) waitMinimumPeersForSync() {
	for {
		meta, err := s.getSelfMeta()
		if err != nil {
			panic("Can't fetch local metadata")
		}

		localBlockNum := meta.HeadBlockNum
		peers, targetBlockNum := s.findPeersForSync(localBlockNum)
		if len(peers) >= minPeers || targetBlockNum <= localBlockNum {
			break
		}

		log.Debugf("Waiting for peers. Need: %d. Now: %d", minPeers, len(peers))
		time.Sleep(5 * time.Second)
	}
}

func (s *Service) syncWithNetwork() error {
	meta, err := s.getSelfMeta()
	if err != nil {
		return errors.New("Can't fetch local metadata")
	}
	startBlockNum := meta.HeadBlockNum
	if startBlockNum == s.cfg.P2P.PeerStore().Scorers().PeerHeadBlock.Get() {
		log.Info("Node is already synced")
		s.pushSyncedState()
		return nil
	}

	// wait for peers to syncing
	s.waitMinimumPeersForSync()

	peers, targetBlockNum := s.findPeersForSync(startBlockNum)

	// local node is in front of all chain
	if targetBlockNum <= startBlockNum {
		s.pushSyncedState()
		return nil
	}

	if len(peers) < minPeers {
		return errors.New("Not enough peers to sync")
	}

	requestsCount := (targetBlockNum - startBlockNum) % blocksPerRequest

	// skip existed blocks except Genesis
	if startBlockNum > 0 {
		startBlockNum++
	}

	// todo filter peers by block scorer
	request := &prototype.BlockRequest{
		StartSlot: startBlockNum,
		Count:     blocksPerRequest,
		Step:      1,
	}

	blocks, err := s.requestBlocks(request, peers)
	if err != nil {
		return err
	}

	log.Debugf("Receive blocks: %d", len(blocks))

	// process blocks
	for _, b := range blocks {
		err := s.cfg.Storage.FinalizeBlock(b)
		if err != nil {
			return errors.Wrap(err, "Error processing block")
		}
	}

	if requestsCount > 1 {
		s.pushSyncingState()
	} else {
		s.pushSyncedState()
	}

	return nil
}

func (s *Service) requestBlocks(request *prototype.BlockRequest, peers []peer.ID) ([]*prototype.Block, error) {
	for i := 0; i < len(peers); i++ {
		if blocks, err := s.sendBlockRangeRequest(s.ctx, request, peers[i]); err == nil {
			// todo update peer score
			return blocks, nil
		} else {
			log.Error(err)
		}
	}

	return nil, errors.New("No peers to handle request")
}

func (s *Service) findPeersForSync(localHeadBlockNum uint64) ([]peer.ID, uint64) {
	connected := s.cfg.P2P.PeerStore().Connected()
	peers := make([]peer.ID, 0, len(connected))
	bestBlockNum := make(map[uint64]int, len(connected))
	nums := make(map[peer.ID]uint64, len(connected))

	for _, data := range connected {
		if data.HeadBlockNum > localHeadBlockNum {
			bestBlockNum[data.HeadBlockNum]++
			peers = append(peers, data.Id)
			nums[data.Id] = data.HeadBlockNum
		}
	}

	maxVotes := 0
	var targetBlockNum uint64
	for bnum, count := range bestBlockNum {
		if count > maxVotes || (count == maxVotes && bnum > targetBlockNum) {
			maxVotes = count
			targetBlockNum = bnum
		}
	}

	sort.Slice(peers, func(i, j int) bool {
		if nums[peers[i]] == nums[peers[j]] {
			return false
		}

		return nums[peers[i]] > nums[peers[j]]
	})

	for i, pid := range peers {
		if nums[pid] < targetBlockNum {
			peers = peers[:i]
			break
		}
	}

	return peers, targetBlockNum
}