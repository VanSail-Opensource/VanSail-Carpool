package pricing

import (
	"math"
	"time"
)

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

func (h Health) telemetryFresh(now time.Time) bool {
	return !h.receivedAt.IsZero() && !now.Before(h.receivedAt) && now.Sub(h.receivedAt) <= time.Minute &&
		(h.validUntil.IsZero() || !now.After(h.validUntil))
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
	if !old.At.IsZero() && o.At.Before(old.At) {
		return old
	}
	h.At = o.At
	h.ReportedAt = o.ReportedAt
	sampleAt := o.ReceivedAt
	if sampleAt.IsZero() {
		sampleAt = o.ReportedAt
	}
	h.receivedAt = sampleAt
	h.validUntil = o.ValidUntil
	fresh := !o.Stale && h.telemetryFresh(o.At)
	usable := fresh && o.Connected && o.Ready && o.Synchronized && o.Complete && !o.Maintenance
	continuous := old.telemetryFresh(sampleAt) && (old.State == "HEALTHY" || old.State == "DEGRADED" || old.State == "OVERLOADED" || old.State == "RECOVERING")
	if !usable || !continuous {
		h.warningAt, h.criticalAt, h.peakAt, h.recoveryAt = time.Time{}, time.Time{}, time.Time{}, time.Time{}
		h.highSamples = 0
	}
	if usable && (!continuous || sampleAt.After(old.receivedAt)) {
		alpha := 1.0
		if continuous {
			alpha = 1 - math.Exp(-max(1, sampleAt.Sub(old.receivedAt).Seconds())/60)
		}
		h.CPU = old.CPU + alpha*(o.CPU-old.CPU)
		h.Memory = o.Memory
		if o.CPU >= 85 || o.Memory >= 92 {
			h.highSamples++
		} else {
			h.highSamples = 0
		}
	}
	warn := persistent(&h.warningAt, usable && o.CPU >= 85, sampleAt, 3*time.Minute) && h.highSamples >= 3
	critical := persistent(&h.criticalAt, usable && (o.CPU >= 95 || o.Memory >= 92), sampleAt, 2*time.Minute) && h.highSamples >= 3
	peak := persistent(&h.peakAt, usable && o.CPU >= 99, sampleAt, time.Minute) && h.highSamples >= 3
	state := "HEALTHY"
	offline := persistent(&h.offlineAt, !o.Connected, o.At, time.Minute)
	switch {
	case o.Maintenance:
		state = "MAINTENANCE"
	case offline:
		state = "OFFLINE"
	case !fresh:
		state = "TELEMETRY_STALE"
	case critical || peak:
		state = "OVERLOADED"
	case !o.Complete || !o.Connected || !o.Ready || !o.Synchronized || warn || o.Memory >= 92:
		state = "DEGRADED"
	}
	if state == "HEALTHY" && old.State != "HEALTHY" && old.State != "" {
		if h.recoveryAt.IsZero() {
			h.recoveryAt = o.At
		}
		if sampleAt.Sub(h.recoveryAt) < 5*time.Minute {
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
