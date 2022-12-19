package sync

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/blockchain/consensus"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"sort"
	"time"
)

const waitPeerInterval = 5 * time.Second

func (s *Service) waitMinimumPeersForSync(findWithMaxBlock bool) {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			peers, _ := s.findPeers(findWithMaxBlock)

			if len(peers) >= s.cfg.MinSyncPeers {
				return
			}

			log.Infof("Waiting for enough peers. Need: %d. Now: %d.", s.cfg.MinSyncPeers, len(peers))
			time.Sleep(waitPeerInterval)
		}
	}
}

func (s *Service) syncWithNetwork() error {
	// sync to most known block
	err := s.syncToBestKnownBlock()
	if err != nil {
		return err
	}

	// node is synced
	if s.cfg.Blockchain.GetHeadBlockNum() == s.cfg.P2P.PeerStore().Scorers().PeerHeadBlock.Get() {
		return nil
	}

	// sync to max known block
	return s.syncToMaxBlock()
}

func (s *Service) filterPeers(peers []peer.ID) {
	store := s.cfg.P2P.PeerStore()
	sort.Slice(peers, func(i, j int) bool {
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
			log.Error(errors.Wrap(err, "Block request error"))
			log.Errorf("Peers len %d", len(peers))
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
	scorer := s.cfg.P2P.PeerStore().Scorers().PeerHeadBlock
	remoteHeadBlockNum := scorer.Get()

	for i := 0; i < 5 && !scorer.Initialized(); i++ {
		log.Debug("Wait for fair meta update")
		time.Sleep(5000 * time.Millisecond)
	}

	if startBlockNum >= remoteHeadBlockNum {
		log.Info("Node is already synced")
		return nil
	}

	// wait for peers to synced
	s.waitMinimumPeersForSync(false)

	peers, targetBlockNum := s.findPeersForSync(startBlockNum)

	// local node is in front of all chain
	if targetBlockNum <= startBlockNum {
		return nil
	}

	return s.requestData(startBlockNum, targetBlockNum, peers, false)
}

func (s *Service) syncToMaxBlock() error {
	// wait for peers to synced
	s.waitMinimumPeersForSync(true)

	startBlockNum := s.cfg.Blockchain.GetHeadBlockNum()
	peers, targetBlockNum := s.findPeersForSyncWithMaxBlock()

	return s.requestData(startBlockNum, targetBlockNum, peers, true)
}

func (s *Service) requestData(startBlockNum, targetBlockNum uint64, peers []peer.ID, maxBlockMode bool) error {
	if len(peers) == 0 {
		return errors.New("No peers to sync")
	}

	f := NewFetcher(s.ctx, startBlockNum, targetBlockNum, s, maxBlockMode)

	for {
		select {
		case <-s.ctx.Done():
			return nil
		case res, ok := <-f.Response():
			if !ok {
				return nil
			}

			for _, b := range res.blocks {
				err := s.cfg.Storage.FinalizeBlock(b)
				if err != nil && !errors.Is(err, consensus.ErrKnownBlock) {
					s.cancel()
					return errors.Wrap(err, "Error processing block batch")
				}

				log.Infof("Sync and save block %d / %d", b.Num, targetBlockNum)
			}
		case err := <-f.Error():
			return err
		}
	}
}

func (s *Service) findPeers(findWithMaxBlock bool) ([]peer.ID, uint64) {
	var peers []peer.ID
	var targetBlockNum uint64
	if findWithMaxBlock {
		peers, targetBlockNum = s.findPeersForSyncWithMaxBlock()
	} else {
		localBlockNum := s.cfg.Blockchain.GetHeadBlockNum()
		peers, targetBlockNum = s.findPeersForSync(localBlockNum)
	}

	return peers, targetBlockNum
}
