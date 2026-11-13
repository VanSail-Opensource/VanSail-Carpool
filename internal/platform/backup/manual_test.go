package backup

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type manualBackupRepository struct {
	actor     string
	admitErr  error
	completed bool
}

func (r *manualBackupRepository) StartBackupRun(context.Context, string, time.Time) (model.BackupRun, error) {
	return model.BackupRun{}, errors.New("manual backup used unaudited admission")
}

func (r *manualBackupRepository) StartAuditedBackupRun(_ context.Context, path, actorID string, now time.Time) (model.BackupRun, error) {
	r.actor = actorID
	if r.admitErr != nil {
		return model.BackupRun{}, r.admitErr
	}
	return model.BackupRun{ID: "manual", Path: path, Status: "running", CreatedAt: now}, nil
}

func (r *manualBackupRepository) CompleteBackupRun(context.Context, string, int64, error, time.Time) error {
	r.completed = true
	return nil
}

func TestManualBackupRequiresAuditedAdmission(t *testing.T) {
	for _, unavailable := range []bool{false, true} {
		t.Run(map[bool]string{false: "failed export remains attributed", true: "audit failure prevents export"}[unavailable], func(t *testing.T) {
			repository := &manualBackupRepository{}
			if unavailable {
				repository.admitErr = errors.New("audit admission unavailable")
			}
			// An unconfigured exporter fails without starting a host utility.
			service := NewService(nil, repository, t.TempDir(), time.Hour)
			run, err := service.RunForActor(context.Background(), "admin")
			if err == nil || repository.actor != "admin" {
				t.Fatal("manual backup lost admission or actor", run, err, repository.actor)
			}
			if unavailable {
				if !errors.Is(err, repository.admitErr) || run.ID != "" || repository.completed {
					t.Fatal("failed audit reached export/completion", run, err)
				}
			} else if run.ID != "manual" || run.Status != "failed" || !repository.completed {
				t.Fatal("failed export lost its admitted attempt", run, err)
			}
		})
	}
}

func TestManualBackupRejectsUnauditedRepositoryAndBlankActor(t *testing.T) {
	called := false
	legacy := &canceledBackupRepository{cancel: func() { called = true }}
	service := NewService(nil, legacy, t.TempDir(), time.Hour)
	if _, err := service.RunForActor(context.Background(), "admin"); err == nil || called {
		t.Fatal("legacy repository bypassed mandatory audit", err)
	}
	repository := &manualBackupRepository{}
	service = NewService(nil, repository, t.TempDir(), time.Hour)
	if _, err := service.RunForActor(context.Background(), " "); err == nil || repository.actor != "" || repository.completed {
		t.Fatal("blank actor started a manual backup", err)
	}
}
