package backup

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type auditedBackupRepository interface {
	StartAuditedBackupRun(context.Context, string, string, time.Time) (model.BackupRun, error)
}

// Run records a scheduled backup before starting its export.
func (s *Service) Run(ctx context.Context) (model.BackupRun, error) {
	return s.run(ctx, func(path string, now time.Time) (model.BackupRun, error) {
		return s.repository.StartBackupRun(ctx, path, now)
	})
}

// RunForActor requires the attempt and actor audit to commit before any export.
func (s *Service) RunForActor(ctx context.Context, actorID string) (model.BackupRun, error) {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return model.BackupRun{}, errors.New("backup actor is required")
	}
	repository, ok := s.repository.(auditedBackupRepository)
	if !ok {
		return model.BackupRun{}, errors.New("audited backup admission is unavailable")
	}
	return s.run(ctx, func(path string, now time.Time) (model.BackupRun, error) {
		return repository.StartAuditedBackupRun(ctx, path, actorID, now)
	})
}
