package backup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type canceledBackupRepository struct {
	cancel             context.CancelFunc
	completionContext  error
	completionDeadline time.Time
	completed          bool
	exportError        error
	completionError    error
}

func (r *canceledBackupRepository) StartBackupRun(_ context.Context, path string, at time.Time) (model.BackupRun, error) {
	r.cancel()
	return model.BackupRun{ID: "canceled", Path: path, CreatedAt: at}, nil
}

func (r *canceledBackupRepository) CompleteBackupRun(ctx context.Context, _ string, _ int64, exportErr error, _ time.Time) error {
	r.completed = true
	r.completionContext = ctx.Err()
	r.completionDeadline, _ = ctx.Deadline()
	r.exportError = exportErr
	return r.completionError
}

func TestBackupCompletionSurvivesAttemptCancellation(t *testing.T) {
	t.Parallel()
	for _, failCompletion := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		repository := &canceledBackupRepository{cancel: cancel}
		if failCompletion {
			repository.completionError = errors.New("metadata unavailable")
		}
		// An unconfigured exporter fails without invoking any host utility.
		service := NewService(nil, repository, t.TempDir(), time.Hour)
		run, err := service.Run(ctx)
		cancel()
		if err == nil || run.Status != "failed" || repository.exportError == nil {
			t.Fatal("failed export was not reported", run.Status, err)
		}
		if !repository.completed || repository.completionContext != nil || repository.completionDeadline.IsZero() {
			t.Fatal("cancellation prevented bounded outcome persistence", repository.completionContext)
		}
		if time.Until(repository.completionDeadline) > 5*time.Second {
			t.Fatal("completion context is not bounded")
		}
		if failCompletion && !errors.Is(err, repository.completionError) {
			t.Fatal("lost metadata persistence failure", err)
		}
	}
}
