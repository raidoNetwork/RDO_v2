package slot

import (
	"github.com/pkg/errors"
	"github.com/raidoNetwork/RDO_v2/shared/params"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

var mainTicker *SlotTicker
var log = logrus.WithField("prefix", "SlotTicker")

const SlotsPerEpoch = 200

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

	slotDuration time.Duration
	slotSec int64

	done chan struct{}
	c chan uint64

	mu sync.Mutex
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

	var nextTickTime time.Time
	timePassed := time.Since(genesisTime)
	if timePassed < st.slotDuration {
		nextTickTime = genesisTime
	} else{
		nextTick := timePassed.Truncate(st.slotDuration) + st.slotDuration
		nextTickTime = genesisTime.Add(nextTick)
	}

	// count current slot
	st.slot = st.currentSlot(genesisTime)

	// count current epoch
	st.epoch = st.currentEpoch()

	go func() {
		for {
			waitTime := time.Until(nextTickTime)

			select {
				case <-time.After(waitTime):
					st.c <- st.slot

					st.mu.Lock()
					st.slot++
					if st.slot % SlotsPerEpoch == 0 && st.slot > 0 {
						st.epoch++
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
	return st.slot / SlotsPerEpoch
}