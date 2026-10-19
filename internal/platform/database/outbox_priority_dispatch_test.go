package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
)

func TestFirewallReservedDispatchContinuesWhileProviderWaits(t *testing.T) {
	s := newTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now().UTC().Add(-time.Second)
	insert := func(id, kind, payload string) {
		t.Helper()
		if _, err := s.db.ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,attempts,available_at,created_at,updated_at) VALUES(?,?,?,'pending',0,?,?,?)`, id, kind, payload, stamp(now), stamp(now), stamp(now)); err != nil {
			t.Fatal(err)
		}
	}
	insert("provider", "provider_operation", `{}`)
	if job, err := s.ClaimPriorityOutboxJob(ctx, now.Add(time.Second)); err != nil || job != nil {
		t.Fatalf("reserved path claimed ordinary work: %+v %v", job, err)
	}
	worker := outbox.NewWorker(s)
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	if err := worker.Register("provider_operation", outbox.HandlerFunc(func(ctx context.Context, _ model.OutboxJob) error {
		close(started)
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})); err != nil {
		t.Fatal(err)
	}
	delivered := false
	if err := worker.Register("xnode_firewall", outbox.HandlerFunc(func(context.Context, model.OutboxJob) error { delivered = true; return nil })); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- worker.Drain(ctx, 1) }()
	defer func() { cancel(); <-done }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("provider never started")
	}
	insert("urgent", "xnode_firewall", `{"nodeId":"node","bank":"dynamic"}`)
	if err := worker.DrainPriority(ctx, 2); err != nil || !delivered {
		t.Fatal("firewall admission waited for provider", err)
	}
	if job, err := s.ClaimOutboxJob(ctx, time.Now().UTC()); err != nil || job != nil {
		t.Fatalf("completed or leased job was claimed twice: %+v %v", job, err)
	}
}
