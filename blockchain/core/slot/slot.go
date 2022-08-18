package slot

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"sync"
	"time"
)

var mainTicker *SlotTicker

func NewSlotTicker(){
	slotDuration := params.RaidoConfig().SlotTime

	ticker := &SlotTicker{
		slot: 0,
		epoch: 0,
		done: make(chan struct{}),
		c: make(chan uint64),
		slotDuration: time.Duration(slotDuration) * time.Second,
		slotSec: slotDuration,
	}

	mainTicker = ticker
}

func Ticker() *SlotTicker {
	return mainTicker
}

type SlotTicker struct{
	slot uint64
	epoch uint64
	startEpochSlot uint64
	lastEpochSlot uint64

	slotDuration time.Duration
	slotSec int64

	done chan struct{}
	c chan uint64

	mu sync.Mutex

	genesisTime time.Time
}

func (st *SlotTicker) C() <-chan uint64 {
	return st.c
}

func (st *SlotTicker) Stop(){
	go func(){
		st.done <- struct{}{}
	}()
}

func (st *SlotTicker) Start(genesisTime time.Time) error {
	if genesisTime.IsZero() {
		return errors.New("zero Genesis time")
	}

	st.mu.Lock()
	st.genesisTime = genesisTime
	st.mu.Unlock()

	var nextTickTime time.Time
	timePassed := time.Since(genesisTime)
	if timePassed < st.slotDuration {
		nextTickTime = genesisTime
	} else {
		nextTick := timePassed.Truncate(st.slotDuration) + st.slotDuration
		nextTickTime = genesisTime.Add(nextTick)
	}

	slotsPerEpoch := params.RaidoConfig().SlotsPerEpoch

	st.mu.Lock()
	st.genesisTime = genesisTime

	// count current slot
	st.slot = st.currentSlot(genesisTime)

	// count current epoch
	st.epoch = st.currentEpoch()

	// slots data
	st.startEpochSlot = st.epoch * slotsPerEpoch
	st.lastEpochSlot = st.startEpochSlot + slotsPerEpoch

	st.mu.Unlock()

	go func() {
		for {
			waitTime := time.Until(nextTickTime)

			select {
			case <-time.After(waitTime):
				st.c <- st.slot

				st.mu.Lock()
				st.slot++

				if st.slot % slotsPerEpoch == 0 && st.slot > 0 {
					st.epoch++
					st.startEpochSlot = st.slot
					st.lastEpochSlot = st.slot + slotsPerEpoch
				}
				st.mu.Unlock()

				nextTickTime = nextTickTime.Add(st.slotDuration)
			case <-st.done:
				return
			}
		}
	}()


	return nil
}

func (st *SlotTicker) Slot() uint64 {
	st.mu.Lock()
	defer st.mu.Unlock()

	return st.slot
}

func (st *SlotTicker) Epoch() uint64 {
	st.mu.Lock()
	defer st.mu.Unlock()

	return st.epoch
}

func (st *SlotTicker) currentSlot(genesisTime time.Time) uint64 {
	now := time.Now().Unix()
	genesisSec := genesisTime.Unix()

	if now < genesisSec {
		return 0
	}

	return uint64((now - genesisSec) / st.slotSec)
}

func (st *SlotTicker) currentEpoch() uint64 {
	return st.slot / params.RaidoConfig().SlotsPerEpoch
}

func (st *SlotTicker) IsLastEpochSlot() bool {
	st.mu.Lock()
	defer st.mu.Unlock()

	return st.slot == st.lastEpochSlot
}

func (st *SlotTicker) GenesisAfter() bool {
	st.mu.Lock()
	defer st.mu.Unlock()

	return st.genesisTime.After(time.Now())
}