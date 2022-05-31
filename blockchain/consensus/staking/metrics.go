package staking

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	stakeSlotsFilled = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "stake_slots_filled",
		Help: "Filled stake slots count",
	})
)
