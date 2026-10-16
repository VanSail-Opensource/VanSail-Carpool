package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// TemporaryBan creates a durable, time-bounded connection restriction.
func (s *UserWorkflows) TemporaryBan(ctx context.Context, actorID, userID, key, reason string, durationMinutes int) (model.OperationReceipt, error) {
	reason = strings.TrimSpace(reason)
	if !validCommand(actorID, key, reason) || strings.TrimSpace(userID) == "" || durationMinutes < 1 || durationMinutes > 525600 {
		return model.OperationReceipt{}, errors.New("invalid temporary ban")
	}
	fingerprint, err := commandFingerprint(struct {
		UserID, Reason string
		Duration       int
	}{userID, reason, durationMinutes})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	return s.repository.CreateAdminTemporaryBan(ctx, database.AdminTemporaryBanInput{ActorUserID: actorID, UserID: userID,
		IdempotencyKey: strings.TrimSpace(key), RequestFingerprint: fingerprint, Reason: reason, DurationMinutes: durationMinutes}, s.now().UTC())
}

// TemporaryUnban queues an immediate restoration without modifying entitlement dates.
func (s *UserWorkflows) TemporaryUnban(ctx context.Context, actorID, userID, key, reason string) (model.OperationReceipt, error) {
	reason = strings.TrimSpace(reason)
	if !validCommand(actorID, key, reason) || strings.TrimSpace(userID) == "" {
		return model.OperationReceipt{}, errors.New("invalid temporary unban")
	}
	fingerprint, err := commandFingerprint(map[string]string{"userId": userID, "reason": reason})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	return s.repository.CreateAdminTemporaryUnban(ctx, actorID, userID, strings.TrimSpace(key), fingerprint, reason, s.now().UTC())
}
