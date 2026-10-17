package xnode

import (
	"context"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"time"
)

type operationLeaseRepository interface {
	RenewOutboxJob(context.Context, string, int, time.Time) (bool, error)
	CompleteOutboxJob(context.Context, string, int, error, time.Time) error
}

// DeferOperation releases the durable claim loop while a long command awaits
// the agent. Frames still use the existing send queue. Leases remain in SQLite,
// heartbeat during the wait, and recover normally after a process restart.
func (m *Master) DeferOperation(job model.OutboxJob, execute func(context.Context) error) error {
	return m.deferOperation(job, 5*time.Minute, execute)
}
func (m *Master) deferOperation(job model.OutboxJob, timeout time.Duration, execute func(context.Context) error) error {
	repo, ok := m.service.repo.(operationLeaseRepository)
	if !ok {
		return ErrUnavailable
	}
	m.mu.Lock()
	if m.ctx == nil || m.ctx.Err() != nil || m.operations >= 4 {
		m.mu.Unlock()
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(m.ctx, timeout)
	m.operations++
	m.wg.Add(1)
	m.mu.Unlock()
	go func() {
		defer m.wg.Done()
		defer cancel()
		defer func() { m.mu.Lock(); m.operations--; m.mu.Unlock() }()
		heartbeatDone := make(chan struct{})
		go func() {
			defer close(heartbeatDone)
			tick := time.NewTicker(20 * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
					owned, err := repo.RenewOutboxJob(ctx, job.ID, job.Attempts, m.service.now().UTC())
					if err != nil || !owned {
						cancel()
						return
					}
				}
			}
		}()
		owned, err := repo.RenewOutboxJob(ctx, job.ID, job.Attempts, m.service.now().UTC())
		if err == nil && !owned {
			err = ErrConflict
		}
		if err == nil {
			err = execute(ctx)
		}
		cancel()
		<-heartbeatDone
		finish, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer stop()
		if completionErr := repo.CompleteOutboxJob(finish, job.ID, job.Attempts, err, m.service.now().UTC()); completionErr != nil && !errors.Is(completionErr, context.Canceled) {
			m.logger.Warn("deferred node operation remains leased", "error_code", "XNODE_OPERATION_COMPLETION_PENDING")
		}
	}()
	return outbox.ErrDeferred
}
