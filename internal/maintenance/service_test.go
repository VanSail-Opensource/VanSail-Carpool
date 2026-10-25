package maintenance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestRunBacksUpBeforeCompactingOncePerLocalDate(t *testing.T) {
	location := time.FixedZone("CST", 8*60*60)
	repository := &maintenanceRepository{runID: "run-1", acquired: true}
	backup := &maintenanceBackup{run: model.BackupRun{ID: "backup-1", Status: "complete"}}
	service := NewService(repository, backup, location)
	at := time.Date(2026, 8, 18, 17, 0, 0, 0, time.UTC)
	if _, err := service.Run(context.Background(), at); err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if len(repository.calls) != 1 || repository.calls[0] != "compact" || backup.calls != 1 {
		t.Fatalf("order calls=%v backup=%d", repository.calls, backup.calls)
	}
	if repository.date != "2026-08-19" {
		t.Fatalf("local date=%q", repository.date)
	}
	if repository.backupID != "backup-1" || repository.counts["payments"] != 3 {
		t.Fatalf("completion backup=%q counts=%v", repository.backupID, repository.counts)
	}
}

func TestRunSkipsCompactionWhenBackupFails(t *testing.T) {
	repository := &maintenanceRepository{runID: "run-1", acquired: true}
	backup := &maintenanceBackup{err: errors.New("disk full")}
	service := NewService(repository, backup, time.UTC)
	if _, err := service.Run(context.Background(), time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("Run() unexpectedly succeeded")
	}
	if len(repository.calls) != 0 || repository.completedErr == nil {
		t.Fatalf("calls=%v completedErr=%v", repository.calls, repository.completedErr)
	}
}

func TestRunRecordsFailureWhenWorkerContextExpires(t *testing.T) {
	repository := &maintenanceRepository{runID: "run-1", acquired: true}
	backup := &maintenanceBackup{err: context.DeadlineExceeded}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewService(repository, backup, time.UTC).Run(ctx, time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline exceeded", err)
	}
	if repository.completedErr == nil || repository.completionContextErr != nil {
		t.Fatalf("completed error=%v completion context error=%v", repository.completedErr, repository.completionContextErr)
	}
}

func TestRunReturnsWithoutWorkWhenDateLeaseIsHeld(t *testing.T) {
	repository := &maintenanceRepository{acquired: false}
	service := NewService(repository, &maintenanceBackup{}, time.UTC)
	if _, err := service.Run(context.Background(), time.Now()); err != nil {
		t.Fatalf("Run(): %v", err)
	}
	if repository.claims != 1 {
		t.Fatalf("claims=%d", repository.claims)
	}
}

func TestRunManualForcesSameDayRerun(t *testing.T) {
	repository := &maintenanceRepository{runID: "run-2", acquired: true}
	backup := &maintenanceBackup{run: model.BackupRun{ID: "backup-2", Status: "complete"}}
	service := NewService(repository, backup, time.UTC)
	if _, err := service.RunManual(context.Background(), time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("RunManual(): %v", err)
	}
	if !repository.force {
		t.Fatal("RunManual() did not request a forced claim")
	}
}

func TestRunManualReportsHeldLease(t *testing.T) {
	repository := &maintenanceRepository{runID: "run-1", acquired: false}
	service := NewService(repository, &maintenanceBackup{}, time.UTC)
	if _, err := service.RunManual(context.Background(), time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)); !errors.Is(err, ErrBusy) {
		t.Fatalf("RunManual() error = %v, want ErrBusy", err)
	}
}

func TestRunCompactsAfterVerifiedBackupRetentionWarning(t *testing.T) {
	repository := &maintenanceRepository{runID: "run-1", acquired: true}
	backup := &maintenanceBackup{run: model.BackupRun{ID: "backup-1", Status: "complete"}, err: errors.New("remove expired backup")}
	result, err := NewService(repository, backup, time.UTC).Run(context.Background(), time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC))
	if err != nil || result.BackupRetentionWarning == nil {
		t.Fatalf("Run() = (%+v, %v)", result, err)
	}
	if len(repository.calls) != 1 || repository.completedErr != nil {
		t.Fatalf("calls=%v completedErr=%v", repository.calls, repository.completedErr)
	}
}

type maintenanceRepository struct {
	runID                string
	acquired             bool
	claims               int
	date                 string
	backupID             string
	counts               map[string]int64
	completedErr         error
	completionContextErr error
	calls                []string
	compactErr           error
	completionErr        error
	force                bool
}

func (r *maintenanceRepository) ClaimMaintenanceRun(_ context.Context, date, _ string, _, _ time.Time, force bool) (string, bool, error) {
	r.claims++
	r.date = date
	r.force = force
	return r.runID, r.acquired, nil
}
func (r *maintenanceRepository) CompactAndPrune(context.Context, time.Time, time.Time, time.Time) (map[string]int64, error) {
	r.calls = append(r.calls, "compact")
	if r.counts == nil {
		r.counts = map[string]int64{"payments": 3}
	}
	return r.counts, r.compactErr
}
func (r *maintenanceRepository) CompleteMaintenanceRun(ctx context.Context, _, backupID string, counts map[string]int64, runErr error, _ time.Time) error {
	r.backupID, r.counts, r.completedErr = backupID, counts, runErr
	r.completionContextErr = ctx.Err()
	return r.completionErr
}

func (r *maintenanceRepository) PrunePAYGTransientRecords(context.Context, time.Time) (map[string]int64, error) {
	return map[string]int64{"xnode_payg_minutes": 2}, nil
}

type maintenanceBackup struct {
	run   model.BackupRun
	err   error
	calls int
}

func (b *maintenanceBackup) Run(context.Context) (model.BackupRun, error) {
	b.calls++
	return b.run, b.err
}
