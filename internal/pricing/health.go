package pricing

import (
	"context"
	"sync"
	"time"
)

// Observation contains transient authenticated machine telemetry, never SQL data.
type Observation struct {
	NodeID                                             string
	At                                                 time.Time
	ReportedAt                                         time.Time
	ReceivedAt                                         time.Time
	ValidUntil                                         time.Time
	Connected, Ready, Synchronized, Stale, Maintenance bool
	CPU, Memory                                        float64
	Complete                                           bool
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
	validUntil                                           time.Time
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

// Snapshot copies transient projections and treats old observations as unavailable.
func (r *HealthRegistry) Snapshot(now time.Time) []Health {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Health, 0, len(r.nodes))
	for _, h := range r.nodes {
		if now.Sub(h.At) > time.Minute || (h.State != "OFFLINE" && h.State != "MAINTENANCE" && !h.telemetryFresh(now)) {
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
		if !ok || h.State != "HEALTHY" || now.Sub(h.At) > time.Minute || !h.telemetryFresh(now) {
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
