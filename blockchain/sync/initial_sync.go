package sync

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"sort"
	"time"
)

func (s *Service) waitMinimumPeersForSync(findWithMaxBlock bool) {
	var peers []peer.ID
	var targetBlockNum uint64
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			localBlockNum := s.cfg.Blockchain.GetHeadBlockNum()
			if findWithMaxBlock {
				peers, targetBlockNum = s.findPeersForSyncWithMaxBlock()
			} else {
				peers, targetBlockNum = s.findPeersForSync(localBlockNum)
			}

			if len(peers) >= minPeers || targetBlockNum <= localBlockNum {
				return
			}

			log.Debugf("Waiting for peers. Need: %d. Now: %d", minPeers, len(peers))
			time.Sleep(5 * time.Second)
		}
	}
}

func (s *Service) syncWithNetwork() error {
	err := s.syncToBestKnownBlock()
	if err != nil {
		return err
	}

	// node is synced
	if s.cfg.Blockchain.GetHeadBlockNum() == s.cfg.P2P.PeerStore().Scorers().PeerHeadBlock.Get() {
		return nil
	}

	return s.syncToMaxBlock()
}

func (s *Service) filterPeers(peers []peer.ID) {
	store := s.cfg.P2P.PeerStore()
	sort.Slice(peers, func (i, j int) bool {
		return store.BlockRequestCounter(peers[i]) < store.BlockRequestCounter(peers[j]) && !store.IsBad(peers[i])
	})
}

func (s *Service) requestBlocks(request *prototype.BlockRequest, peers []peer.ID) ([]*prototype.Block, error) {
	// filter peers
	s.filterPeers(peers)

	for _, peer := range peers {
		if blocks, err := s.sendBlockRangeRequest(s.ctx, request, peer); err == nil {
			s.cfg.P2P.PeerStore().AddBlockParse(peer)
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


func (s *Service) findPeersForSyncWithMaxBlock() ([]peer.ID, uint64) {
	connected := s.cfg.P2P.PeerStore().Connected()
	maxBlock := s.cfg.P2P.PeerStore().Scorers().PeerHeadBlock.Get()
	peers := make([]peer.ID, 0, len(connected))

	for _, data := range connected {
		if data.HeadBlockNum == maxBlock {
			peers = append(peers, data.Id)
		}
	}

	return peers, maxBlock
}

func (s *Service) syncToBestKnownBlock() error {
	startBlockNum := s.cfg.Blockchain.GetHeadBlockNum()
	if startBlockNum == s.cfg.P2P.PeerStore().Scorers().PeerHeadBlock.Get() {
		log.Info("Node is already synced")
		return nil
	}

	// wait for peers to syncing
	s.waitMinimumPeersForSync(false)

	peers, targetBlockNum := s.findPeersForSync(startBlockNum)

	// local node is in front of all chain
	if targetBlockNum <= startBlockNum {
		return nil
	}

	if len(peers) < minPeers {
		return errors.New("Not enough peers to sync")
	}

	f := NewFetcher(s.ctx, startBlockNum, targetBlockNum, s, peers)

	for res := range f.Response() {
		for _, b := range res.blocks {
			err := s.cfg.Storage.FinalizeBlock(b)
			if err != nil && !errors.Is(err, consensus.ErrKnownBlock) {
				s.cancel()
				return errors.Wrap(err, "Error processing block batch")
			}

			log.Infof("Sync and save block %d / %d", b.Num, targetBlockNum)
		}
	}

	return nil
}

func (s *Service) syncToMaxBlock() error {
	startBlockNum := s.cfg.Blockchain.GetHeadBlockNum()

	// wait for peers to syncing
	s.waitMinimumPeersForSync(true)

	peers, targetBlockNum := s.findPeersForSyncWithMaxBlock()

	if len(peers) < minPeers {
		return errors.New("Not enough peers to sync")
	}

	f := NewFetcher(s.ctx, startBlockNum, targetBlockNum, s, peers)

	for res := range f.Response() {
		for _, b := range res.blocks {
			err := s.cfg.Storage.FinalizeBlock(b)
			if err != nil && !errors.Is(err, consensus.ErrKnownBlock) {
				s.cancel()
				return errors.Wrap(err, "Error processing block batch")
			}

			log.Infof("Sync and save block %d / %d", b.Num, targetBlockNum)
		}
	}

	return nil
}