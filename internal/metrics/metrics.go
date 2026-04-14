// Package metrics provides Prometheus instrumentation for the Clawgress gateway.
//
// Exposes counters, histograms, and gauges for request flow, policy decisions,
// quota state, and system health. Scrape via GET /metrics on the admin API.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts proxy requests by agent, decision, and policy.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "clawgress",
		Subsystem: "gateway",
		Name:      "requests_total",
		Help:      "Total proxy requests by agent, decision, and policy.",
	}, []string{"agent_id", "decision", "policy_id"})

	// RequestDuration tracks proxy request latency in seconds.
	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "clawgress",
		Subsystem: "gateway",
		Name:      "request_duration_seconds",
		Help:      "Proxy request duration in seconds.",
		Buckets:   []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5},
	}, []string{"agent_id", "decision"})

	// BytesOut tracks outbound bytes through the proxy.
	BytesOut = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "clawgress",
		Subsystem: "gateway",
		Name:      "bytes_out_total",
		Help:      "Total bytes forwarded outbound by agent.",
	}, []string{"agent_id"})

	// QuotaUsage tracks current quota utilization (0-1 scale).
	QuotaUsage = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "clawgress",
		Subsystem: "quota",
		Name:      "utilization_ratio",
		Help:      "Current quota utilization ratio (0=empty, 1=full).",
	}, []string{"agent_id", "limit_type"})

	// ActiveAgents tracks the number of registered agents.
	ActiveAgents = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "clawgress",
		Subsystem: "identity",
		Name:      "active_agents",
		Help:      "Number of active registered agents.",
	})

	// PolicyRules tracks the number of loaded policy rules.
	PolicyRules = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "clawgress",
		Subsystem: "policy",
		Name:      "rules_total",
		Help:      "Number of loaded policy rules.",
	})

	// AuditEventsTotal counts audit events written.
	AuditEventsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "clawgress",
		Subsystem: "audit",
		Name:      "events_total",
		Help:      "Total audit events written.",
	})

	// DenyTotal counts denied requests (convenience counter).
	DenyTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "clawgress",
		Subsystem: "gateway",
		Name:      "deny_total",
		Help:      "Total denied requests by reason.",
	}, []string{"reason"})
)
