// Package maintenance coordinates the daily backup-gated retention pass.
package maintenance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const (
	maintenanceCompletionTimeout = 5 * time.Second
	maintenanceLease             = RunTimeout + maintenanceCompletionTimeout
)

// RunTimeout bounds one scheduled or manual online backup and retention pass.
// The lease remains valid slightly longer so a timed-out run can record its
// failure without another process starting overlapping maintenance.
const RunTimeout = 45 * time.Minute

// ErrBusy indicates that another maintenance run still owns the active lease.
var ErrBusy = errors.New("maintenance is already running")

// Repository owns the daily lease and one atomic compact-and-prune transaction.
type Repository interface {
	ClaimMaintenanceRun(context.Context, string, string, time.Time, time.Time, bool) (string, bool, error)
	CompactAndPrune(context.Context, time.Time, time.Time, time.Time) (map[string]int64, error)
	PrunePAYGTransientRecords(context.Context, time.Time) (map[string]int64, error)
	CompleteMaintenanceRun(context.Context, string, string, map[string]int64, error, time.Time) error
}

// BackupRunner returns a failed or incomplete backup on verification failure. A
// complete backup may carry a post-publication retention warning.
type BackupRunner interface {
	Run(context.Context) (model.BackupRun, error)
}

// RunResult separates a verified backup's non-blocking retention warning from
// the backup-gated cleanup transaction result.
type RunResult struct {
	BackupRetentionWarning error
}

// Service is safe to call from a scheduler that may receive overlapping ticks.
type Service struct {
	repository Repository
	backups    BackupRunner
	location   *time.Location
	owner      string
	now        func() time.Time
	mu         sync.Mutex
}

// NewService creates a daily maintenance coordinator.
func NewService(repository Repository, backups BackupRunner, location *time.Location) *Service {
	if location == nil {
		location = time.UTC
	}
	return &Service{repository: repository, backups: backups, location: location,
		owner: fmt.Sprintf("tx-carpool-maintenance-%d", os.Getpid()), now: time.Now}
}

// Run claims the configured local date, verifies a backup, then compacts and
// purges records. A failed backup never reaches the purge transaction. A
// successful run for the local date is not repeated by the scheduler.
func (s *Service) Run(ctx context.Context, at time.Time) (RunResult, error) {
	return s.run(ctx, at, false)
}

// RunManual performs the complete maintenance flow even when the local date
// already has a successful run. An active lease remains exclusive.
func (s *Service) RunManual(ctx context.Context, at time.Time) (RunResult, error) {
	return s.run(ctx, at, true)
}

func (s *Service) run(ctx context.Context, at time.Time, force bool) (RunResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := RunResult{}
	now := at
	if now.IsZero() {
		now = s.now()
	}
	now = now.UTC()
	local := now.In(s.location)
	date := local.Format(time.DateOnly)
	runID, acquired, err := s.repository.ClaimMaintenanceRun(ctx, date, s.owner, now.Add(maintenanceLease), now, force)
	if err != nil {
		return result, err
	}
	if !acquired {
		if force {
			return result, ErrBusy
		}
		return result, nil
	}
	backup, backupErr := s.backups.Run(ctx)
	if backup.Status != "complete" {
		if backupErr == nil {
			backupErr = errors.New("backup did not complete verification")
		}
		return result, errors.Join(backupErr, s.complete(ctx, runID, backup.ID, nil, backupErr, now))
	}
	result.BackupRetentionWarning = backupErr
	samples, sampleErr := s.repository.PrunePAYGTransientRecords(ctx, now)
	counts, compactErr := s.repository.CompactAndPrune(ctx, now.Add(-7*24*time.Hour), now.Add(-24*time.Hour), now)
	if counts == nil {
		counts = map[string]int64{}
	}
	for key, count := range samples {
		counts[key] += count
	}
	compactErr = errors.Join(sampleErr, compactErr)
	return result, errors.Join(compactErr, s.complete(ctx, runID, backup.ID, counts, compactErr, now))
}

func (s *Service) complete(ctx context.Context, runID, backupID string, counts map[string]int64, runErr error, now time.Time) error {
	completionCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), maintenanceCompletionTimeout)
	defer cancel()
	return s.repository.CompleteMaintenanceRun(completionCtx, runID, backupID, counts, runErr, now)
}
