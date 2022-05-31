package core

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/raidoNetwork/RDO_v2/blockchain/core/slot"
)

var (
	headSlot = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "head_slot",
		Help: "Current slot of the network",
	})
	headEpoch = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "head_epoch",
		Help: "Current epoch of the network",
	})
)

func updateCoreMetrics() {
	ticker := slot.Ticker()

	headSlot.Set(float64(ticker.Slot()))
	headEpoch.Set(float64(ticker.Epoch()))
}