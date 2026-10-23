package connectivity

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
)

func (s *Service) process(item *work) {
	status, code := "completed", ""
	defer func() { s.finishRun(item, status, code) }()
	if item.ctx.Err() != nil {
		status, code = "interrupted", "CONNECTIVITY_INTERRUPTED"
		return
	}
	_, targets, err := s.loadTargets(item.ctx, item.config, item.hash, true)
	if err != nil {
		status, code = "error", ErrorCode(err)
		if item.ctx.Err() != nil {
			status, code = "interrupted", "CONNECTIVITY_INTERRUPTED"
		}
		if saveErr := s.recordSetupFailure(item, status, code); saveErr != nil {
			status, code = "error", "CONNECTIVITY_STORAGE_UNAVAILABLE"
		}
		return
	}
	if len(targets) == 0 {
		status, code = "error", "CONNECTIVITY_NO_ACCESSIBLE_HOSTS"
		if err := s.recordSetupFailure(item, status, code); err != nil {
			code = "CONNECTIVITY_STORAGE_UNAVAILABLE"
		}
		return
	}
	s.mu.Lock()
	item.run.Total = len(targets)
	s.mu.Unlock()
	for _, target := range targets {
		if item.ctx.Err() != nil {
			status, code = "interrupted", "CONNECTIVITY_INTERRUPTED"
			return
		}
		if err := s.checkTarget(item, target); err != nil {
			status, code = "error", "CONNECTIVITY_STORAGE_UNAVAILABLE"
			if errors.Is(err, context.Canceled) && item.ctx.Err() != nil {
				status, code = "interrupted", "CONNECTIVITY_INTERRUPTED"
			}
			return
		}
		s.mu.Lock()
		item.run.Completed++
		s.mu.Unlock()
	}
	if item.ctx.Err() != nil {
		status, code = "interrupted", "CONNECTIVITY_INTERRUPTED"
	}
}

func (s *Service) newAttempt(item *work, hostUUID string) Attempt {
	return Attempt{ID: uuid.NewString(), RunID: item.run.ID, HostUUID: hostUUID, ConfigHash: item.hash,
		ProxyUserID: item.config.ProxyUserID, Trigger: item.run.Trigger, StartedAt: s.now().UTC(),
		Outcome: Outcome{Status: "running"}}
}

func (s *Service) checkTarget(item *work, target Target) error {
	attempt := s.newAttempt(item, target.HostUUID)
	if err := s.repository.BeginConnectivityAttempt(item.ctx, attempt); err != nil {
		return err
	}
	outcome := s.checkWithRetries(item, target)
	if item.ctx.Err() != nil {
		outcome = interruptedOutcome()
	}
	return s.finishAttempt(item.ctx, attempt.ID, outcome)
}

func (s *Service) finishAttempt(ctx context.Context, id string, outcome Outcome) error {
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storageTimeout)
	defer cancel()
	return s.repository.FinishConnectivityAttempt(finishCtx, id, outcome, s.now().UTC())
}

func (s *Service) recordSetupFailure(item *work, status, code string) error {
	attempt := s.newAttempt(item, "")
	ctx, cancel := context.WithTimeout(context.WithoutCancel(item.ctx), storageTimeout)
	defer cancel()
	if err := s.repository.BeginConnectivityAttempt(ctx, attempt); err != nil {
		return err
	}
	return s.repository.FinishConnectivityAttempt(ctx, attempt.ID, Outcome{Status: status, ErrorCode: code}, s.now().UTC())
}

func (s *Service) finishRun(item *work, status, code string) {
	finished := s.now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	item.run.Status, item.run.ErrorCode, item.run.FinishedAt = status, code, &finished
	if s.active == item {
		s.active = nil
	}
	if item.generation == s.generation {
		copy := cloneRun(item.run)
		s.last, s.lastHash, s.completed = &copy, item.hash, finished
	}
}

func sanitizedOutcome(outcome Outcome) Outcome {
	switch outcome.Status {
	case "connected", "failed", "unsupported", "error", "interrupted":
	default:
		return Outcome{Status: "error", ErrorCode: "CONNECTIVITY_INVALID_OUTCOME"}
	}
	if outcome.ErrorCode != "" && !safeErrorCode.MatchString(outcome.ErrorCode) {
		outcome.ErrorCode = "CONNECTIVITY_UNAVAILABLE"
	}
	if outcome.LatencyMS != nil && (*outcome.LatencyMS < 0 || math.IsNaN(*outcome.LatencyMS) || math.IsInf(*outcome.LatencyMS, 0)) {
		outcome.LatencyMS = nil
	}
	if outcome.HTTPStatus != nil && (*outcome.HTTPStatus < 100 || *outcome.HTTPStatus > 599) {
		outcome.HTTPStatus = nil
	}
	return outcome
}
