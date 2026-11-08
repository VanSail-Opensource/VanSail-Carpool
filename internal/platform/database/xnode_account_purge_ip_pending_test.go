package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPurgeIPReceiptRetainsUncertainMutationAndProtectsPromotion(t *testing.T) {
	s, target, jobID, control := nativePurgeIPFixture(t)
	ctx := context.Background()
	at := stamp(time.Now().UTC())
	_, err := s.db.ExecContext(ctx, `INSERT INTO provider_operations(id,actor_user_id,owner_user_id,kind,idempotency_key,request_fingerprint,status,attempts,attempt_started_at,created_at,updated_at)
 VALUES('uncertain-block',$1,$1,'connection_block','uncertain-block',$2,'pending_review',1,$3,$3,$3)`, target.UserID, strings.Repeat("c", 64), at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.RecordAccountPurgeIPClear(ctx, target, jobID, control); !errors.Is(err, xnode.ErrAccessAccountingPending) {
		t.Fatal("uncertain future block discarded", err)
	}
	var status string
	if err = s.db.QueryRowContext(ctx, `SELECT status FROM provider_operations WHERE id='uncertain-block'`).Scan(&status); err != nil || status != "pending_review" {
		t.Fatal("cleanup rewrote uncertain outcome", status, err)
	}
	if _, err = s.db.ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, target.UserID); err != nil {
		t.Fatal(err)
	}
	if applied, e := s.RecordAccountPurgeIPClear(ctx, target, jobID, control); e != nil || applied {
		t.Fatal("admin promotion admitted IP cleanup", applied, e)
	}
	var rows int
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE action='account.purge_ip_cleared'`).Scan(&rows); err != nil || rows != 0 {
		t.Fatal("refused clear wrote native evidence", rows, err)
	}
}
