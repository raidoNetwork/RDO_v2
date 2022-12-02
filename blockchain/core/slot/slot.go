package slot

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/params"
)

var mainTicker *SlotTicker

func CreateSlotTicker() {
	mainTicker = NewSlotTicker()
}

func NewSlotTicker() *SlotTicker {
	slotDuration := params.RaidoConfig().SlotTime

	return &SlotTicker{
		slot:         0,
		epoch:        0,
		done:         make(chan struct{}),
		c:            make(chan uint64),
		slotDuration: time.Duration(slotDuration) * time.Second,
		slotSec:      slotDuration,
	}
}

func Ticker() *SlotTicker {
	return mainTicker
}

type SlotTicker struct {
	slot           uint64
	epoch          uint64
	startEpochSlot uint64
	lastEpochSlot  uint64

	slotDuration time.Duration
	slotSec      int64

	done chan struct{}
	c    chan uint64

	mu sync.Mutex

	genesisTime time.Time
}

func (st *SlotTicker) C() <-chan uint64 {
	return st.c
}

func (st *SlotTicker) Stop() {
	go func() {
		st.done <- struct{}{}
	}()
}

func (st *SlotTicker) StartFromTimestamp(tstamp uint64) error {
	timeFormat := time.Unix(0, int64(tstamp))
	return st.Start(timeFormat)
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
		nextTick := timePassed.Round(st.slotDuration) + st.slotDuration
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

				if st.slot%slotsPerEpoch == 0 && st.slot > 0 {
					st.epoch++
					st.startEpochSlot = st.slot
					st.lastEpochSlot = st.slot + slotsPerEpoch
					go checkClockDrift()
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
	now := time.Now()

	if now.Before(genesisTime) {
		return 0
	}

	elapsed := now.Sub(genesisTime)
	roundedSeconds := elapsed.Round(st.slotDuration).Seconds()

	return uint64(int64(roundedSeconds) / st.slotSec)
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

func (st *SlotTicker) GenesisTime() time.Time {
	st.mu.Lock()
	defer st.mu.Unlock()

	return st.genesisTime
}
