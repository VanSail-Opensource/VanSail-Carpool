package xnode

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// Called with Master.mu held. These are admission slots in the existing durable
// executor, never another queue. Member work cannot consume the firewall reserve.
func (m *Master) reserveOperation(job model.OutboxJob, member bool) bool {
	if member {
		if m.memberOperations >= 1 {
			return false
		}
		m.memberOperations++
		return true
	}
	var payload struct {
		Bank string `json:"bank"`
	}
	limit := 3
	if job.Kind == FirewallJob && json.Unmarshal([]byte(job.Payload), &payload) == nil && payload.Bank == "dynamic" {
		limit = 4
	}
	if m.operations >= limit {
		return false
	}
	m.operations++
	return true
}

func runDeferredOperation(ctx context.Context, execute func(context.Context) error) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("deferred node operation panicked")
		}
	}()
	return execute(ctx)
}
