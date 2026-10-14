package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

// ResolveOperation records an audited administrator decision for an ambiguous result.
func (s *UserWorkflows) ResolveOperation(ctx context.Context, actorID, operationID, key,
	resolution, reason string) (model.OperationReceipt, error) {
	resolution = strings.TrimSpace(resolution)
	reason = strings.TrimSpace(reason)
	if !validCommand(actorID, key, reason) || strings.TrimSpace(operationID) == "" ||
		!validOperationResolution(resolution) {
		return model.OperationReceipt{}, errors.New("invalid operation resolution")
	}
	fingerprint, err := commandFingerprint(struct {
		OperationID string `json:"operationId"`
		Resolution  string `json:"resolution"`
		Reason      string `json:"reason"`
	}{strings.TrimSpace(operationID), resolution, reason})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	return s.repository.ResolveAdminOperation(ctx, database.AdminOperationResolutionInput{
		ActorUserID: actorID, OperationID: strings.TrimSpace(operationID), IdempotencyKey: strings.TrimSpace(key),
		RequestFingerprint: fingerprint, Resolution: resolution, Reason: reason,
	}, s.now().UTC())
}

func validOperationResolution(value string) bool {
	return value == "succeeded" || value == "failed" || value == "compensated" || value == "partial"
}
