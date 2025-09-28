package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	WebSocketConnections = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "websocket_connections_total",
			Help: "Number of active WebSocket connections",
		},
		[]string{"user_id"},
	)

	DatabaseQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "db_query_duration_seconds",
			Help: "Database query duration",
		},
		[]string{"query_type"},
	)

	WebSocketEventDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "websocket_event_duration_seconds",
			Help: "WebSocket event processing duration",
		},
		[]string{"event_type"},
	)
)
