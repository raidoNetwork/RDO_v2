package p2p

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	p2pPeerMap = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "p2p_peer_map",
		Help: "The number of peers in a given state.",
	},
		[]string{"state"},
	)
	totalPeerCount = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "p2p_peer_count",
		Help: "Tracks the total number of peers",
	})
)

func (s *Service) updateMetrics() {
	peers := s.peerStore.Stats()
	totalPeerCount.Set(float64(peers["total"]))

	p2pPeerMap.WithLabelValues("Connected").Set(float64(peers["connected"]))
	p2pPeerMap.WithLabelValues("Reconnected").Set(float64(peers["reconnected"]))
	p2pPeerMap.WithLabelValues("Disconnected").Set(float64(peers["disconnected"]))
}

