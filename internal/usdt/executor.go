package usdt

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

// Executor reuses upstreamqueue for each provider-account budget; it is not a
// competing queue. Connection lifetimes and inbound readers live outside workers.
type Executor struct {
	repository Repository
	mu         sync.Mutex
	queues     map[string]*upstreamqueue.Queue
	groups     map[string]QuotaGroup
	ctx        context.Context
}

func newExecutor(repository Repository) *Executor {
	return &Executor{repository: repository, queues: map[string]*upstreamqueue.Queue{}, groups: map[string]QuotaGroup{}}
}

func (e *Executor) configure(ctx context.Context, groups []QuotaGroup) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, q := range e.queues {
		if err := q.Shutdown(ctx); err != nil {
			return err
		}
		delete(e.queues, id)
	}
	e.ctx = ctx
	e.groups = map[string]QuotaGroup{}
	for _, g := range groups {
		q, err := upstreamqueue.New(upstreamqueue.Config{Name: "usdt:" + g.ID, Capacity: 32, MinInterval: time.Second / time.Duration(g.RequestsPerSecond), Workers: g.Concurrency})
		if err != nil {
			return err
		}
		if err := q.Start(ctx); err != nil {
			return err
		}
		e.queues[g.ID] = q
		e.groups[g.ID] = g
	}
	return nil
}

func (e *Executor) close(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	var errs []error
	for _, q := range e.queues {
		errs = append(errs, q.Shutdown(ctx))
	}
	e.queues = map[string]*upstreamqueue.Queue{}
	return errors.Join(errs...)
}

func (e *Executor) do(ctx context.Context, group string, call func(context.Context) error) error {
	e.mu.Lock()
	q, g := e.queues[group], e.groups[group]
	e.mu.Unlock()
	if q == nil {
		return ErrUnavailable
	}
	return upstreamqueue.Execute(ctx, q, func(jobCtx context.Context) error {
		admitErr := e.repository.AdmitUSDTRPC(jobCtx, g, time.Now().UTC())
		var admission *Pause
		if errors.As(admitErr, &admission) && admission.Code == "USDT_RPC_COOLDOWN" && time.Until(admission.Until) > 0 && time.Until(admission.Until) < 100*time.Millisecond {
			timer := time.NewTimer(time.Until(admission.Until))
			defer timer.Stop()
			select {
			case <-jobCtx.Done():
				return jobCtx.Err()
			case <-timer.C:
			}
			admitErr = e.repository.AdmitUSDTRPC(jobCtx, g, time.Now().UTC())
		}
		if admitErr != nil {
			return admitErr
		}
		callCtx, cancel := context.WithTimeout(jobCtx, 15*time.Second)
		defer cancel()
		err := call(callCtx)
		var pause *Pause
		if errors.As(err, &pause) {
			if saveErr := e.repository.FinishUSDTRPC(jobCtx, group, pause.Until, false); saveErr != nil {
				return saveErr
			}
			return err
		}
		if err != nil {
			if jobCtx.Err() != nil {
				return jobCtx.Err()
			}
			until := time.Now().UTC().Add(time.Duration(rand.IntN(1000)) * time.Millisecond)
			if saveErr := e.repository.FinishUSDTRPC(jobCtx, group, until, false); saveErr != nil {
				return saveErr
			}
			return err
		}
		return e.repository.FinishUSDTRPC(jobCtx, group, time.Time{}, true)
	})
}
