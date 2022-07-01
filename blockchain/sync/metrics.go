package sync

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	receivedMessages = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "p2p_message_received",
			Help: "Count of messages received.",
		},
		[]string{"topic"},
	)
	messageFailedProcessingCounter = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "p2p_failed_message_received",
			Help: "Count of failed messages received.",
		},
		[]string{"topic"},
	)
)
