package database

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func nativePurgeIPFixture(t *testing.T) (*Store, model.AccountPurge, string, xnode.AccountPurgeIPControl) {
	t.Helper()
	s, target, jobID, _ := nativePurgeFixture(t)
	ctx := context.Background()
	var node string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM xnode_nodes ORDER BY id LIMIT 1`).Scan(&node); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO connection_ip_blocks(id,user_id,node_uuid,ip_digest,sealed_ip,status,expiry_job_id,created_at,updated_at,expires_at)
 VALUES('purge-sensitive-block',$1,$2,$3,'opaque-encrypted-target','active','retained-expiry-job',$4,$4,$5)`, target.UserID, node, strings.Repeat("a", 64), stamp(at.Add(-8*24*time.Hour)), stamp(at.Add(-time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	controls, active, err := s.AccountPurgeIPControls(ctx, target, jobID)
	if err != nil || !active || len(controls) != 1 {
		t.Fatal("IP control not captured", controls, active, err)
	}
	return s, target, jobID, controls[0]
}

func purgeIPReady(t *testing.T, s *Store, target model.AccountPurge) error {
	t.Helper()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	valid, err := s.accountCleanupAdmissionTx(ctx, tx, target.UserID, target.Generation, time.Now().UTC())
	if err != nil || !valid {
		t.Fatal(valid, err)
	}
	if err = fenceAccountPurgeOperationsTx(ctx, tx, target.UserID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	return accountPurgeIPControlsReadyTx(ctx, tx, target.UserID, target.Generation)
}

func TestPurgeIPLocalExpiryCannotSubstituteForNativeReceipt(t *testing.T) {
	s, target, jobID, control := nativePurgeIPFixture(t)
	ctx := context.Background()
	if err := purgeIPReady(t, s, target); err == nil || !strings.Contains(err.Error(), "IP_EVIDENCE_PENDING") {
		t.Fatal("local deadline erased unknown native block", err)
	}
	wrong := control
	wrong.AgentID = "wrong-agent"
	if _, err := s.RecordAccountPurgeIPClear(ctx, target, jobID, wrong); err == nil {
		t.Fatal("wrong agent receipt accepted")
	}
	if ok, err := s.RecordAccountPurgeIPClear(ctx, target, jobID, control); err != nil || !ok {
		t.Fatal("exact internal native receipt rejected", ok, err)
	}
	if err := purgeIPReady(t, s, target); err != nil {
		t.Fatal("exact receipt failed final barrier", err)
	}
	controls, _, err := s.AccountPurgeIPControls(ctx, target, jobID)
	if err != nil || len(controls) != 0 {
		t.Fatal("receipt was not replayable", controls, err)
	}
	var rows int
	var detail string
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM connection_ip_blocks WHERE user_id=$1`, target.UserID).Scan(&rows); err != nil || rows != 1 {
		t.Fatal("native receipt scrubbed encrypted row before final purge", rows, err)
	}
	if err = s.db.QueryRowContext(ctx, `SELECT detail FROM audit_events WHERE action='account.purge_ip_cleared'`).Scan(&detail); err != nil || strings.Contains(detail, "source_ip") || strings.Contains(detail, "opaque-encrypted-target") {
		t.Fatal("receipt leaked target material", detail, err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE connection_ip_blocks SET ip_digest=$1 WHERE id=$2`, strings.Repeat("b", 64), control.Block.ID); err != nil {
		t.Fatal(err)
	}
	if err = purgeIPReady(t, s, target); err == nil {
		t.Fatal("changed encrypted target retained stale clear proof")
	}
}

func TestPurgeIPReceiptKeepsExactExpiryIdentity(t *testing.T) {
	s, target, jobID, control := nativePurgeIPFixture(t)
	ctx := context.Background()
	// Both instants round to the same PostgreSQL microsecond, but the later
	// deadline belongs to a different native control than the captured receipt.
	control.Block.ExpiresAt = control.Block.ExpiresAt.Truncate(time.Second)
	if _, err := s.db.ExecContext(ctx, `UPDATE connection_ip_blocks SET expires_at=$1 WHERE id=$2`, stamp(control.Block.ExpiresAt), control.Block.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.RecordAccountPurgeIPClear(ctx, target, jobID, control); err != nil || !ok {
		t.Fatal(ok, err)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE connection_ip_blocks SET expires_at=$1 WHERE id=$2`, stamp(control.Block.ExpiresAt.Add(time.Nanosecond)), control.Block.ID); err != nil {
		t.Fatal(err)
	}
	if err := purgeIPReady(t, s, target); err == nil {
		t.Fatal("rounded expiry reused a stale native receipt")
	}
	controls, active, err := s.AccountPurgeIPControls(ctx, target, jobID)
	if err != nil || !active || len(controls) != 1 {
		t.Fatal("changed native control disappeared from recovery", controls, active, err)
	}
}
