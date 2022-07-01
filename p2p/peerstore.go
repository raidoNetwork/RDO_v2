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

var PeerMetaUpdateInterval = time.Duration(12 * params.RaidoConfig().SlotTime) * time.Second

type scorers struct {
	PeerHeadSlot *score.Scorer
	PeerHeadBlock *score.Scorer
}

type PeerStore struct {
	data map[string]*PeerData
	lock sync.Mutex
	scorers
}

type MetaData struct {
	HeadSlot uint64
	HeadBlockNum uint64
	HeadBlockHash common.Hash
}

type PeerData struct {
	Id peer.ID
	Status PeerStatus
	LastUpdate time.Time
	MetaData
}

func NewPeerStore() *PeerStore {
	return &PeerStore{
		data: map[string]*PeerData{},
		scorers: scorers{
			PeerHeadSlot: score.MaxScorer(),
			PeerHeadBlock: score.MaxScorer(),
		},
	}
}

func (ps *PeerStore) Connect(id peer.ID) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if pdata, exists := ps.data[id.String()]; exists {
		pdata.Status = Reconnected
		return
	}

	ps.data[id.String()] = &PeerData{
		Id: id,
	}
}


func (ps *PeerStore) Disconnect(id peer.ID) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	pdata, exists := ps.data[id.String()]
	if !exists {
		return
	}

	pdata.Status = Disconnected
}

func (ps *PeerStore) AddMeta(id peer.ID, meta *prototype.Metadata) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	pid := id.String()
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