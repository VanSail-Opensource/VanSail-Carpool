package database

import (
	"context"
	"testing"
	"time"
)

func TestOutboxSkipsOccupiedAccountAcrossKinds(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, row := range []struct{ id, kind, payload string }{
		{"01", "xnode_sync_user", `{"userId":"account-a"}`},
		{"02", "xnode_apply_entitlement", `{"userId":"account-a"}`},
		{"03", "xnode_sync_user", `{"userId":"account-b"}`},
	} {
		_, err := s.db.ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,attempts,available_at,created_at,updated_at) VALUES(?,?,?,'pending',0,?,?,?)`, row.id, row.kind, row.payload, stamp(now), stamp(now), stamp(now))
		if err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || first == nil || first.ID != "01" {
		t.Fatalf("first=%+v %v", first, err)
	}
	second, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || second == nil || second.ID != "03" {
		t.Fatalf("independent=%+v %v", second, err)
	}
	blocked, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || blocked != nil {
		t.Fatalf("account overlapped=%+v %v", blocked, err)
	}
	if err := s.CompleteOutboxJob(ctx, first.ID, first.Attempts, nil, now); err != nil {
		t.Fatal(err)
	}
	next, err := s.ClaimOutboxJob(ctx, now)
	if err != nil || next == nil || next.ID != "02" {
		t.Fatalf("successor=%+v %v", next, err)
	}
}
