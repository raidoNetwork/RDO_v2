package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/raidoNetwork/RDO_v2/proto/prototype"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/raidoNetwork/RDO_v2/shared/score"
	"sync"
	"time"
)

type PeerStatus int8

const (
	Connected PeerStatus = iota
	Disconnected
	Reconnected
)

const badThreshold = 1

var PeerMetaUpdateInterval = time.Duration(4 * params.RaidoConfig().SlotTime) * time.Second

type scorers struct {
	PeerHeadSlot *score.Scorer
	PeerHeadBlock *score.Scorer
}

type PeerStore struct {
	data map[peer.ID]*PeerData
	lock sync.Mutex
	scorers
}

type MetaData struct {
	HeadSlot uint64
	HeadBlockNum uint64
	HeadBlockHash common.Hash
}

type PeerScorers struct {
	BlockRequest int64
	BadResponse int64
}

type PeerData struct {
	Id peer.ID
	Status PeerStatus
	LastUpdate time.Time
	Scorers PeerScorers
	MetaData
}

func NewPeerStore() *PeerStore {
	return &PeerStore{
		data: map[peer.ID]*PeerData{},
		scorers: scorers{
			PeerHeadSlot: score.MaxScorer(),
			PeerHeadBlock: score.MaxScorer(),
		},
	}
}

func (ps *PeerStore) Connect(id peer.ID) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if pdata, exists := ps.data[id]; exists {
		pdata.Status = Reconnected
		return
	}

	ps.data[id] = &PeerData{
		Id: id,
	}
}


func (ps *PeerStore) Disconnect(id peer.ID) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	pdata, exists := ps.data[id]
	if !exists {
		return
	}

	pdata.Status = Disconnected
}

func (ps *PeerStore) AddMeta(pid peer.ID, meta *prototype.Metadata) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	ps.data[pid].LastUpdate = time.Now()
	ps.data[pid].MetaData = MetaData{
		HeadSlot: meta.HeadSlot,
		HeadBlockNum: meta.HeadBlockNum,
		HeadBlockHash: meta.HeadBlockHash,
	}

	ps.PeerHeadSlot.Set(meta.HeadSlot)
	ps.PeerHeadBlock.Set(meta.HeadBlockNum)
}

func (ps *PeerStore) Stats() map[string]int {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	res := map[string]int{
		"connected": 0,
		"reconnected": 0,
		"disconnected": 0,
		"total": len(ps.data),
	}

	for _, data := range ps.data {
		switch data.Status {
		case Connected:
			res["connected"]++
		case Disconnected:
			res["disconnected"]++
		case Reconnected:
			res["reconnected"]++
		}
	}

	return res
}

func (ps *PeerStore) Connected() []PeerData {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	result := make([]PeerData, 0, len(ps.data))
	for _, data := range ps.data {
		if data.Status == Connected || data.Status == Reconnected  {
			result = append(result, *data)
		}
	}

	return result
}

func (ps *PeerStore) Scorers() scorers {
	return ps.scorers
}

func (ps *PeerStore) BlockRequestCounter(pid peer.ID) int64 {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	return ps.data[pid].Scorers.BlockRequest
}

func (ps *PeerStore) AddBlockParse(pid peer.ID) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	ps.data[pid].Scorers.BlockRequest += 1
}

func (ps *PeerStore) BadResponse(pid peer.ID) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	ps.data[pid].Scorers.BadResponse += 1
}

func (ps *PeerStore) IsBad(pid peer.ID) bool {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if _, exists := ps.data[pid]; !exists {
		return false
	}


	return ps.data[pid].Scorers.BadResponse >= badThreshold
}