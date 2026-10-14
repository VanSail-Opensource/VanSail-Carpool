package connectivity

import (
	"context"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func (s *Service) checkWithRetries(item *work, target Target) Outcome {
	for retry := 0; ; retry++ {
		if item.ctx.Err() != nil {
			return interruptedOutcome()
		}
		outcome := s.queuedCheck(item, target)
		if outcome.Status != "failed" || retry >= item.config.MaxRetries {
			return outcome
		}
		if err := s.retryWait(item.ctx, time.Duration(item.config.RetryIntervalSeconds)*time.Second); err != nil {
			return interruptedOutcome()
		}
	}
}

func (s *Service) queuedCheck(item *work, target Target) Outcome {
	probeCtx, cancel := context.WithTimeout(item.ctx, time.Duration(item.config.TimeoutSeconds)*time.Second)
	defer cancel()
	outcome, err := upstreamqueue.Do(probeCtx, s.queue, func(callCtx context.Context) (Outcome, error) {
		return s.probe.Check(callCtx, target, item.config), nil
	})
	if item.ctx.Err() != nil {
		return interruptedOutcome()
	}
	if errors.Is(probeCtx.Err(), context.DeadlineExceeded) || ErrorCode(err) == "CONNECTIVITY_PROBE_TIMEOUT" {
		return Outcome{Status: "failed", ErrorCode: "CONNECTIVITY_PROBE_TIMEOUT"}
	}
	if err != nil {
		return Outcome{Status: "error", ErrorCode: ErrorCode(err)}
	}
	return sanitizedOutcome(outcome)
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}

func interruptedOutcome() Outcome {
	return Outcome{Status: "interrupted", ErrorCode: "CONNECTIVITY_INTERRUPTED"}
}
