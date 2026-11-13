package emby

import (
	"context"
	"errors"
	"fmt"
)

// HandleProvisionJob is the generic outbox handler for ProvisionOutboxKind.
func (s *Service) HandleProvisionJob(ctx context.Context, aggregateID string) error {
	err := s.Provision(ctx, aggregateID)
	if errors.Is(err, ErrPendingReview) {
		return nil
	}
	return err
}

// Provision advances one account through an idempotent remote provisioning saga.
func (s *Service) Provision(ctx context.Context, accountID string) error {
	now := s.now().UTC()
	record, err := s.repository.BeginEmbyProvisioning(ctx, accountID, now)
	if err != nil {
		return err
	}
	if record.Status == StatusActive {
		return nil
	}
	if record.Status == StatusFailed {
		return nil
	}
	if record.Status == StatusPendingReview {
		return ErrPendingReview
	}
	if record.PasswordCiphertext == "" || record.PasswordContext != passwordContext(record.UserID) {
		return s.failTerminal(ctx, record, errors.New("temporary provisioning secret is unavailable"))
	}
	password, err := s.secrets.Open(record.PasswordContext, record.PasswordCiphertext)
	if err != nil {
		return s.failTerminal(ctx, record, fmt.Errorf("open temporary provisioning secret: %w", err))
	}
	defer zero(password)

	remoteUser, err := s.resolveOrCreate(ctx, &record)
	if err != nil {
		return s.handleProvisionError(ctx, record, err)
	}
	if err := s.repository.SetEmbyRemoteIdentity(ctx, record.ID, remoteUser.ID, remoteUser.Name, s.now().UTC()); err != nil {
		return s.handleProvisionError(ctx, record, fmt.Errorf("persist Emby remote identity: %w", err))
	}
	current, err := s.remote.GetUser(ctx, remoteUser.ID)
	if err != nil {
		return s.handleProvisionError(ctx, record, fmt.Errorf("load created Emby user: %w", err))
	}
	if err := s.remote.SetPassword(ctx, current.ID, nil, password); err != nil {
		wrapped := fmt.Errorf("set Emby password: %w", redactRemoteError(err, "remote password update failed"))
		if s.remote.IsTerminal(err) {
			return s.handleProvisionError(ctx, record, wrapped)
		}
		if retryErr := s.repository.RequeueEmbyProvisioning(ctx, record.ID, wrapped, s.now().UTC()); retryErr != nil {
			return fmt.Errorf("record Emby password retry: %w", retryErr)
		}
		return wrapped
	}
	if err := s.remote.UpdatePolicy(ctx, current.ID, HardenPolicy(current.Policy, record.Preferences)); err != nil {
		wrapped := fmt.Errorf("apply restricted Emby policy: %w", err)
		verified, verifyErr := s.remote.GetUser(ctx, current.ID)
		if verifyErr == nil && PolicyMatchesPreferences(verified.Policy, record.Preferences) {
			return s.repository.MarkEmbyProvisioned(ctx, record.ID, record.Preferences, s.now().UTC())
		}
		if s.remote.IsTerminal(err) {
			return s.handleProvisionError(ctx, record, wrapped)
		}
		return s.pendingReview(ctx, record.ID, wrapped)
	}
	verified, err := s.remote.GetUser(ctx, current.ID)
	if err != nil {
		return s.handleProvisionError(ctx, record, fmt.Errorf("verify Emby policy: %w", err))
	}
	if !PolicyMatchesPreferences(verified.Policy, record.Preferences) {
		return s.handleProvisionError(ctx, record, errors.New("Emby policy verification failed"))
	}
	if err := s.repository.MarkEmbyProvisioned(ctx, record.ID, record.Preferences, s.now().UTC()); err != nil {
		return fmt.Errorf("complete Emby provisioning: %w", err)
	}
	return nil
}

func (s *Service) pendingReview(ctx context.Context, accountID string, cause error) error {
	safe := redactRemoteError(cause, "Emby provisioning requires administrator review.")
	if err := s.repository.MarkEmbyPendingReview(ctx, accountID, safe, s.now().UTC()); err != nil {
		return fmt.Errorf("record Emby pending review: %w", err)
	}
	return fmt.Errorf("%w: %v", ErrPendingReview, safe)
}
