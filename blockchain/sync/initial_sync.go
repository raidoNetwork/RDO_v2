package sync

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"sort"
	"time"
)

func (s *Service) waitMinimumPeersForSync() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			localBlockNum := s.cfg.Blockchain.GetHeadBlockNum()
			peers, targetBlockNum := s.findPeersForSync(localBlockNum)
			if len(peers) >= minPeers || targetBlockNum <= localBlockNum {
				return
			}

			log.Debugf("Waiting for peers. Need: %d. Now: %d", minPeers, len(peers))
			time.Sleep(5 * time.Second)
		}
	}
}

func (s *Service) syncWithNetwork() error {
	startBlockNum := s.cfg.Blockchain.GetHeadBlockNum()
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

	request := &prototype.BlockRequest{
		StartSlot: startBlockNum,
		Count:     blocksPerRequest,
		Step:      1,
	}

	blocks, err := s.requestBlocks(request, peers)
	if err != nil {
		return err
	}

	endSlot := request.StartSlot + request.Step * request.Count

	// process blocks
	for _, b := range blocks {
		err := s.cfg.Storage.FinalizeBlock(b)
		if err != nil && !errors.Is(err, consensus.ErrKnownBlock) {
			log.Errorf("Error processing block batch: %s", err)
			break
		}

		log.Infof("Sync and save block %d / %d", b.Num, endSlot)
	}

	if requestsCount > 1 {
		s.pushSyncingState()
	} else {
		if s.cfg.P2P.PeerStore().Scorers().PeerHeadBlock.Get() == s.cfg.Blockchain.GetHeadBlockNum() {
			s.pushSyncedState()
		}
	}

	return nil
}

func (s *Service) filterPeers(peers []peer.ID) []peer.ID {
	store := s.cfg.P2P.PeerStore()
	sort.Slice(peers, func (i, j int) bool {
		return store.BlockRequestCounter(peers[i]) < store.BlockRequestCounter(peers[j]) && !store.IsBad(peers[i])
	})
	return peers
}

func (s *Service) requestBlocks(request *prototype.BlockRequest, peers []peer.ID) ([]*prototype.Block, error) {
	// filter peers
	s.filterPeers(peers)

	for i := 0; i < len(peers); i++ {
		if blocks, err := s.sendBlockRangeRequest(s.ctx, request, peers[i]); err == nil {
			s.cfg.P2P.PeerStore().AddBlockParse(peers[i])
			return blocks, nil
		} else {
			log.Error(errors.Wrap(err, "Error block request"))
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