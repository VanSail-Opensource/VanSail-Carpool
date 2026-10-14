package purchaseops

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

const automaticTrafficResetKeyPrefix = "automatic-traffic-reset:"

var errTrafficResetAutomationUnavailable = errors.New("traffic reset automation is unavailable")

type trafficResetAutomationRepository interface {
	TrafficResetAutomation(context.Context, string) (model.TrafficResetAutomation, error)
	SetTrafficResetAutomation(context.Context, string, bool, time.Time) (model.TrafficResetAutomation, error)
}

// TrafficResetAutomation returns the account-wide automatic reset preference.
func (s *Service) TrafficResetAutomation(ctx context.Context, userID string) (model.TrafficResetAutomation, error) {
	repository, ok := s.repository.(trafficResetAutomationRepository)
	if !ok {
		return model.TrafficResetAutomation{}, errTrafficResetAutomationUnavailable
	}
	return repository.TrafficResetAutomation(ctx, userID)
}

// SetTrafficResetAutomation persists the account-wide automatic reset preference.
func (s *Service) SetTrafficResetAutomation(ctx context.Context, userID string, enabled bool) (model.TrafficResetAutomation, error) {
	repository, ok := s.repository.(trafficResetAutomationRepository)
	if !ok {
		return model.TrafficResetAutomation{}, errTrafficResetAutomationUnavailable
	}
	return repository.SetTrafficResetAutomation(ctx, userID, enabled, s.now().UTC())
}

// AutomaticTrafficResetCommand creates the canonical scanner-owned reset command.
func AutomaticTrafficResetCommand(userID, purchaseID, period string) providerops.CreateInput {
	key := automaticTrafficResetKeyPrefix + strings.TrimSpace(purchaseID) + ":" + strings.TrimSpace(period)
	return command(userID, purchaseID, OperationResetKind, key, operationFingerprint(OperationResetKind, purchaseID))
}

// IsAutomaticTrafficResetKey identifies scanner-owned traffic reset operations.
func IsAutomaticTrafficResetKey(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), automaticTrafficResetKeyPrefix)
}
