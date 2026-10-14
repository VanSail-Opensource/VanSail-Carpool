package admin

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

const MaxExtensionMinutes = 3650 * 24 * 60

// PreviewBulkExtension counts current active targets and all queued successors.
func (s *UserWorkflows) PreviewBulkExtension(ctx context.Context, request BulkExtension) (database.AdminBulkExtensionPreview, error) {
	filter, err := s.normalizeBulkFilter(ctx, request)
	if err != nil {
		return database.AdminBulkExtensionPreview{}, err
	}
	if request.DurationMinutes < 1 || request.DurationMinutes > MaxExtensionMinutes {
		return database.AdminBulkExtensionPreview{}, errors.New("extension minutes are outside the supported range")
	}
	return s.repository.PreviewAdminBulkExtension(ctx, filter, s.now().UTC())
}

// CreateBulkExtension shifts each deduplicated active user exactly once.
func (s *UserWorkflows) CreateBulkExtension(ctx context.Context, actorID, key string, request BulkExtension) (model.OperationReceipt, error) {
	request.Reason = strings.TrimSpace(request.Reason)
	if !validCommand(actorID, key, request.Reason) || request.DurationMinutes < 1 || request.DurationMinutes > MaxExtensionMinutes {
		return model.OperationReceipt{}, errors.New("invalid bulk extension")
	}
	filter, err := s.normalizeBulkFilter(ctx, request)
	if err != nil {
		return model.OperationReceipt{}, err
	}
	fingerprint, err := commandFingerprint(struct {
		Filter          database.AdminBulkExtensionFilter
		DurationMinutes int
		Reason          string
	}{filter, request.DurationMinutes, request.Reason})
	if err != nil {
		return model.OperationReceipt{}, err
	}
	return s.repository.CreateAdminBulkExtension(ctx, database.AdminBulkExtensionInput{
		ActorUserID: actorID, IdempotencyKey: strings.TrimSpace(key), RequestFingerprint: fingerprint,
		Reason: request.Reason, Filter: filter, DurationMinutes: request.DurationMinutes,
	}, s.now().UTC())
}

func (s *UserWorkflows) normalizeBulkFilter(ctx context.Context, request BulkExtension) (database.AdminBulkExtensionFilter, error) {
	comboIDs, err := normalizeIDs(request.ComboIDs)
	if err != nil {
		return database.AdminBulkExtensionFilter{}, err
	}
	addons, err := normalizeUUIDs(request.AddonSquadUUIDs)
	if err != nil {
		return database.AdminBulkExtensionFilter{}, err
	}
	if len(comboIDs) == 0 && len(addons) == 0 {
		return database.AdminBulkExtensionFilter{}, errors.New("at least one bulk extension filter is required")
	}
	for _, comboID := range comboIDs {
		if _, err := s.repository.ComboByID(ctx, comboID, false); err != nil {
			return database.AdminBulkExtensionFilter{}, err
		}
	}
	return database.AdminBulkExtensionFilter{ComboIDs: comboIDs, AddonSquadUUIDs: addons}, nil
}
