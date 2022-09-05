package forger

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/raidoNetwork/RDO_v2/shared/common"
	"time"
)

var (
	pendingTxCounter = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pending_tx_pool_count",
		Help: "Transactions waiting in the pool",
	}) // todo remove to pool metrics
	forgedBlocksCounter = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "forge_block_count",
		Help: "Forged blocks count",
	})
	forgeBlockTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "forge_block_time",
		Help: "Forge new block time",
		Buckets: common.MillisecondsBuckets,
	})
)

func updateForgeMetrics(dur time.Duration) {
	forgedBlocksCounter.Inc()
	forgeBlockTime.Observe(float64(dur.Milliseconds()))
}