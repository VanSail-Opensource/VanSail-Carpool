package postgres_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
)

func TestRecoveryFingerprintsOutboxClaimFence(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	const at = "2026-11-12T00:00:00Z"
	if _, err := db.ExecContext(ctx, `INSERT INTO outbox_jobs(id,kind,payload,status,available_at,created_at,updated_at)
 VALUES('recovery-fence','recovery_fixture','{}','pending',$1,$1,$1)`, at); err != nil {
		t.Fatal(err)
	}
	before, err := postgres.InspectRecovery(ctx, db, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE outbox_jobs SET claim_generation=claim_generation+1 WHERE id='recovery-fence'`); err != nil {
		t.Fatal(err)
	}
	after, err := postgres.InspectRecovery(ctx, db, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	if before.Hash == after.Hash || !reflect.DeepEqual(before.SequenceStates, after.SequenceStates) {
		t.Fatal("changed durable claim fence was not isolated in the recovery fingerprint")
	}
	for index, digest := range before.Tables {
		if digest.Name == "outbox_jobs" {
			if digest.Hash == after.Tables[index].Hash {
				t.Fatal("outbox digest omitted the claim generation")
			}
		} else if digest != after.Tables[index] {
			t.Fatal("claim-only update changed another table digest", digest.Name)
		}
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE outbox_jobs DROP COLUMN claim_generation`); err != nil {
		t.Fatal(err)
	}
	if err := postgres.VerifySchema(ctx, db); err == nil || !strings.Contains(err.Error(), "outbox_jobs.claim_generation") {
		t.Fatal("schema verification accepted a missing durable attempt fence", err)
	}
}
