package p2p

import (
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"sync"
)

type PeerStatus int8

const (
	Connected PeerStatus = iota
	Disconnected
	Reconnected
)

type PeerStore struct {
	data map[string]*PeerData
	lock sync.Mutex
}

type PeerData struct {
	HeadSlot uint64
	HeadBlockNum uint64
	HeadBlockHash common.Hash
	Status PeerStatus
}

func NewPeerStore() *PeerStore {
	return &PeerStore{
		data: map[string]*PeerData{},
	}
}

func (ps *PeerStore) Connect(id peer.ID) {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if pdata, exists := ps.data[id.String()]; exists {
		pdata.Status = Reconnected
	}

	ps.data[id.String()] = &PeerData{}
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
