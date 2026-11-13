package emby

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// remoteAbsenceProof binds an authoritative absence observation to one stored
// setup attempt. A terminal error from password or policy writes proves no such
// absence and must not release the debit for an account that may remain usable.
type remoteAbsenceProof struct {
	record ProvisioningRecord
	cause  error
}

func (e *remoteAbsenceProof) Error() string { return e.cause.Error() }
func (e *remoteAbsenceProof) Unwrap() error { return e.cause }

func (s *Service) currentProvisioningAttempt(ctx context.Context, expected ProvisioningRecord) (ProvisioningRecord, error) {
	current, err := s.repository.EmbyProvisioningByID(ctx, expected.ID)
	if err != nil {
		return ProvisioningRecord{}, fmt.Errorf("load Emby provisioning stage: %w", err)
	}
	if current.ID != expected.ID || current.UserID != expected.UserID || current.SetupAttempt != expected.SetupAttempt {
		return ProvisioningRecord{}, errors.New("Emby provisioning attempt changed")
	}
	return current, nil
}

func (s *Service) confirmRejectedCreate(ctx context.Context, expected ProvisioningRecord, cause error) error {
	current, err := s.currentProvisioningAttempt(ctx, expected)
	if err != nil {
		return err
	}
	if current.RemoteUserID != "" || current.CandidateUsername != expected.CandidateUsername ||
		!current.CreateAttempted || current.Status != StatusProvisioning {
		return cause
	}
	_, exists, err := s.remote.FindUserByName(ctx, current.CandidateUsername)
	if err != nil || exists {
		// A rejected create can race an earlier committed attempt. Do not adopt
		// an account on rejection or assume that the candidate is absent.
		return cause
	}
	return &remoteAbsenceProof{record: current, cause: cause}
}

func (s *Service) handleProvisionError(ctx context.Context, expected ProvisioningRecord, provisionErr error) error {
	if errors.Is(provisionErr, ErrRemoteAccountMissing) || errors.Is(provisionErr, ErrAccountExists) ||
		errors.Is(provisionErr, ErrInvalidSetup) || s.remote.IsTerminal(provisionErr) {
		return s.failTerminal(ctx, expected, provisionErr)
	}
	record, err := s.currentProvisioningAttempt(ctx, expected)
	if err != nil {
		return err
	}
	if record.CreateAttempted || record.RemoteUserID != "" {
		return s.pendingReview(ctx, record.ID, provisionErr)
	}
	if err := s.repository.RequeueEmbyProvisioning(ctx, record.ID, provisionErr, s.now().UTC()); err != nil {
		return fmt.Errorf("record Emby provisioning retry: %w", err)
	}
	return provisionErr
}

func (s *Service) failTerminal(ctx context.Context, expected ProvisioningRecord, cause error) error {
	record, err := s.currentProvisioningAttempt(ctx, expected)
	if err != nil {
		return err
	}
	if record.Status == StatusActive || (record.Status == StatusFailed && record.RefundedAt != nil) {
		return nil
	}
	var proof *remoteAbsenceProof
	absent := errors.As(cause, &proof) && proof.record.ID == record.ID &&
		proof.record.UserID == record.UserID && proof.record.SetupAttempt == record.SetupAttempt &&
		proof.record.UpdatedAt.Equal(record.UpdatedAt) && proof.record.Status == record.Status &&
		proof.record.RemoteUserID == record.RemoteUserID && proof.record.CandidateUsername == record.CandidateUsername &&
		proof.record.CreateAttempted == record.CreateAttempted
	if (record.RemoteUserID != "" || record.CreateAttempted) && !absent {
		return s.pendingReview(ctx, record.ID, cause)
	}
	if writer, ok := s.repository.(interface {
		FailAndRefundEmbySetupIfUnchanged(context.Context, ProvisioningRecord, string, time.Time) (Account, error)
	}); ok {
		_, err = writer.FailAndRefundEmbySetupIfUnchanged(ctx, record, safeFailure(cause), s.now().UTC())
	} else {
		_, err = s.repository.FailAndRefundEmbySetup(ctx, record.ID, safeFailure(cause), s.now().UTC())
	}
	if err != nil {
		return fmt.Errorf("refund failed Emby setup: %w", err)
	}
	return nil
}
