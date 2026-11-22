package app

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type pricingHealthPersistence interface {
	RecordPricingHealth(context.Context, pricing.Health) error
}

type pricingHealthRecord struct {
	state string
	at    time.Time
}

// pricingHealthRecorder belongs to the single pricing worker. It suppresses
// unchanged incident transactions, never the memory-only health observations.
type pricingHealthRecorder struct {
	states map[string]pricingHealthRecord
	cursor int
}

func (r *pricingHealthRecorder) record(ctx context.Context, store pricingHealthPersistence, h pricing.Health) error {
	previous, ok := r.states[h.NodeID]
	if age := h.At.Sub(previous.at); ok && previous.state == h.State && age >= 0 && age < time.Minute {
		return nil
	}
	if err := store.RecordPricingHealth(ctx, h); err != nil {
		// A failed commit can be uncertain. Retry even if the next observation
		// returns to the last successful state, so an open incident is closed.
		delete(r.states, h.NodeID)
		return err
	}
	if r.states == nil {
		r.states = make(map[string]pricingHealthRecord)
	}
	r.states[h.NodeID] = pricingHealthRecord{state: h.State, at: h.At}
	return nil
}

func (r *pricingHealthRecorder) retain(nodes []xnode.NodeMachineMetrics) {
	active := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		active[node.NodeID] = struct{}{}
	}
	for id := range r.states {
		if _, ok := active[id]; !ok {
			delete(r.states, id)
		}
	}
}

// Rotate the first write so a persistently slow incident cannot starve the
// rest of the fleet. This runs after every memory-only observation for the pass.
func (r *pricingHealthRecorder) recordBatch(ctx context.Context, store pricingHealthPersistence, pending []pricing.Health) error {
	if len(pending) == 0 {
		return nil
	}
	var result error
	start := r.cursor % len(pending)
	for i := range pending {
		if ctx.Err() != nil {
			return errors.Join(result, ctx.Err())
		}
		index := (start + i) % len(pending)
		r.cursor = (index + 1) % len(pending)
		result = errors.Join(result, r.record(ctx, store, pending[index]))
	}
	return result
}
