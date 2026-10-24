package xnode

import (
	"sync/atomic"
	"time"
)

type trafficObservations struct{ commits, acknowledged, ackFailures, commitNanos, ackNanos atomic.Uint64 }

// TrafficMetrics contains aggregate process observations, never live user data.
type TrafficMetrics struct {
	Commits, Acknowledged, AckFailures uint64
	CommitSeconds, AckSeconds          float64
}

func (m *Master) TrafficMetrics() TrafficMetrics {
	if m == nil {
		return TrafficMetrics{}
	}
	o := &m.trafficMetrics
	return TrafficMetrics{Commits: o.commits.Load(), Acknowledged: o.acknowledged.Load(), AckFailures: o.ackFailures.Load(), CommitSeconds: float64(o.commitNanos.Load()) / float64(time.Second), AckSeconds: float64(o.ackNanos.Load()) / float64(time.Second)}
}
