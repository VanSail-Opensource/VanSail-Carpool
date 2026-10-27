package pricing

import (
	"context"
	"math"
	"sync"
	"time"
)

// Observation contains transient authenticated machine telemetry, never SQL data.
type Observation struct {
	NodeID                                                               string
	At                                                                   time.Time
	ReportedAt                                                           time.Time
	ReceivedAt                                                           time.Time
	Connected, Ready, Synchronized, Stale, ProbeUnavailable, Maintenance bool
	CPU, Memory                                                          float64
	Complete                                                             bool
}

// Health is a memory-only operational projection with recovery hysteresis.
type Health struct {
	NodeID                                               string    `json:"nodeId"`
	State                                                string    `json:"state"`
	At                                                   time.Time `json:"at"`
	ReportedAt                                           time.Time `json:"reportedAt"`
	CPU                                                  float64   `json:"cpu"`
	Memory                                               float64   `json:"memory"`
	Since                                                time.Time `json:"since"`
	warningAt, criticalAt, peakAt, offlineAt, recoveryAt time.Time
	receivedAt                                           time.Time
	highSamples                                          int
}

// HealthRegistry bounds live state to configured nodes and never persists samples.
type HealthRegistry struct {
	mu                  sync.RWMutex
	nodes               map[string]Health
	pressureUntil       time.Time
	capacityUnsafeUntil map[string]time.Time
}

// SetApplicationPressure gates computation independently of agent health.
func (r *HealthRegistry) SetApplicationPressure(pressured bool, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pressureUntil = time.Time{}
	if pressured {
		r.pressureUntil = now.Add(time.Minute)
	}
}
func (r *HealthRegistry) ApplicationPressured(now time.Time) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return now.Before(r.pressureUntil)
}

func NewHealthRegistry() *HealthRegistry {
	return &HealthRegistry{nodes: map[string]Health{}, capacityUnsafeUntil: map[string]time.Time{}}
}

// Remove forgets disabled nodes without admitting them as healthy.
func (r *HealthRegistry) Remove(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.nodes, node)
	delete(r.capacityUnsafeUntil, node)
}

func persistent(start *time.Time, active bool, now time.Time, duration time.Duration) bool {
	if !active {
		*start = time.Time{}
		return false
	}
	if start.IsZero() {
		*start = now
	}
	return now.Sub(*start) >= duration
}

// Observe uses mean per-core utilization, sustained thresholds and five-minute recovery.
func (r *HealthRegistry) Observe(o Observation) Health {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.nodes[o.NodeID]
	h := old
	h.NodeID = o.NodeID
	if len(r.nodes) >= 4096 && old.NodeID == "" {
		return Health{NodeID: o.NodeID, State: "TELEMETRY_STALE", At: o.At}
	}
	gap := o.At.Sub(old.At)
	if gap > 2*time.Minute {
		h.warningAt = time.Time{}
		h.criticalAt = time.Time{}
		h.peakAt = time.Time{}
		h.recoveryAt = time.Time{}
	}
	alpha := 1 - math.Exp(-max(1, gap.Seconds())/60)
	if old.At.IsZero() {
		alpha = 1
	}
	h.CPU = old.CPU + alpha*(o.CPU-old.CPU)
	h.Memory = o.Memory
	h.At = o.At
	h.ReportedAt = o.ReportedAt
	state := "HEALTHY"
	sampleAt := o.ReceivedAt
	if sampleAt.IsZero() {
		sampleAt = o.ReportedAt
	}
	h.receivedAt = sampleAt
	if !sampleAt.Equal(old.receivedAt) {
		if o.CPU >= 85 || o.Memory >= 92 {
			h.highSamples++
		} else {
			h.highSamples = 0
		}
	}
	warn := persistent(&h.warningAt, o.CPU >= 85, sampleAt, 3*time.Minute) && h.highSamples >= 3
	critical := persistent(&h.criticalAt, o.CPU >= 95 || o.Memory >= 92, sampleAt, 2*time.Minute) && h.highSamples >= 3
	peak := persistent(&h.peakAt, o.CPU >= 99, sampleAt, time.Minute) && h.highSamples >= 3
	offline := persistent(&h.offlineAt, !o.Connected && o.ProbeUnavailable, o.At, time.Minute)
	switch {
	case o.Maintenance:
		state = "MAINTENANCE"
	case offline:
		state = "OFFLINE"
	case o.Stale || sampleAt.IsZero() || o.At.Sub(sampleAt) > time.Minute:
		state = "TELEMETRY_STALE"
	case critical || peak:
		state = "OVERLOADED"
	case !o.Complete || !o.Connected || !o.Ready || !o.Synchronized || warn || o.Memory >= 92:
		state = "DEGRADED"
	}
	if state == "HEALTHY" && old.State != "HEALTHY" && old.State != "" {
		if !persistent(&h.recoveryAt, true, o.At, 5*time.Minute) {
			state = "RECOVERING"
		}
	} else if state != "HEALTHY" {
		h.recoveryAt = time.Time{}
	}
	h.State = state
	if old.State != state {
		h.Since = o.At
	}
	r.nodes[o.NodeID] = h
	return h
}

// Snapshot copies transient projections and treats old observations as unavailable.
func (r *HealthRegistry) Snapshot(now time.Time) []Health {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Health, 0, len(r.nodes))
	for _, h := range r.nodes {
		if now.Sub(h.At) > time.Minute {
			h.State = "TELEMETRY_STALE"
		}
		out = append(out, h)
	}
	return out
}

// Safe requires a fresh healthy observation for every fulfilling node.
func (r *HealthRegistry) Safe(nodes []string, now time.Time) bool {
	return r.safe(nodes, now, false)
}

// TelemetrySafe permits capacity recomputation after a capacity admission pause.
func (r *HealthRegistry) TelemetrySafe(nodes []string, now time.Time) bool {
	return r.safe(nodes, now, true)
}

func (r *HealthRegistry) safe(nodes []string, now time.Time, ignoreCapacity bool) bool {
	if r == nil || len(nodes) == 0 {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, id := range nodes {
		if _, unsafe := r.capacityUnsafeUntil[id]; !ignoreCapacity && unsafe {
			return false
		}
		h, ok := r.nodes[id]
		if !ok || h.State != "HEALTHY" || now.Sub(h.At) > time.Minute {
			return false
		}
	}
	return true
}

// SetCapacity keeps only the transient decision, never connection-derived SQL state.
func (r *HealthRegistry) SetCapacity(node string, unsafe bool, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if unsafe {
		r.capacityUnsafeUntil[node] = now.Add(time.Hour)
	} else {
		delete(r.capacityUnsafeUntil, node)
	}
}

type healthKey struct{}

func WithHealth(ctx context.Context, r *HealthRegistry) context.Context {
	return context.WithValue(ctx, healthKey{}, r)
}
func ContextHealth(ctx context.Context) *HealthRegistry {
	r, _ := ctx.Value(healthKey{}).(*HealthRegistry)
	return r
}
