package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestReservedFirewallAndOrdinaryAdmissionProgressWithoutConsumingBusyAttempts(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, job := range []struct{ id, kind, body string }{{"old", "provider_operation", `{}`}, {"urgent", "xnode_firewall", `{"nodeId":"node","bank":"dynamic"}`}} {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,attempts,available_at,created_at,updated_at) VALUES(?,?,?,'pending',0,?,?,?)`, job.id, job.kind, job.body, stamp(now.Add(-time.Hour)), stamp(now.Add(-time.Hour)), stamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	for range 12 {
		job, err := s.ClaimPriorityOutboxJob(ctx, now)
		if err != nil || job == nil || job.ID != "urgent" || job.Attempts != 1 {
			t.Fatalf("priority/admission = %+v %v", job, err)
		}
		if err = s.CompleteOutboxClaim(ctx, *job, outbox.ErrBusy, now); err != nil {
			t.Fatal(err)
		}
		now = now.Add(2 * time.Second)
	}
	// The same urgent job remains due after repeated busy admissions. It must
	// not starve ordinary work, even when an older firewall job exists.
	job, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || job == nil || job.ID != "old" {
		t.Fatalf("ordinary work starved behind recurring firewall jobs: %+v %v", job, err)
	}
	job, err = s.ClaimPriorityOutboxJob(ctx, now)
	if err != nil || job == nil || job.ID != "urgent" {
		t.Fatalf("firewall reserve lost while ordinary work is leased: %+v %v", job, err)
	}
}
