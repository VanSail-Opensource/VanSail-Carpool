package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
)

func TestRecoveryFingerprintIncludesPopulatedNativeDatesAndTimestamps(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	stamp := "2026-11-12T10:11:12.123456Z"
	queries := []string{
		`INSERT INTO users(id,telegram_id,created_at,updated_at) VALUES('native-time-owner',606060,$1,$1)`,
		`INSERT INTO hourly_credit_days(owner_id,account_generation,day_utc,free_allowance_bytes,created_at,updated_at)
 VALUES('native-time-owner',0,DATE '2026-11-12',0,$1,$1)`,
		`INSERT INTO hourly_account_preferences(owner_id,account_generation,extra_enabled,updated_at)
 VALUES('native-time-owner',0,0,$1)`,
	}
	for _, query := range queries {
		if _, err = db.ExecContext(ctx, query, stamp); err != nil {
			t.Fatal(err)
		}
	}
	inspect := func() postgres.Report {
		t.Helper()
		report, e := postgres.InspectRecovery(ctx, db, make([]byte, 32))
		if e != nil {
			t.Fatal("populated hourly recovery inspection failed", e)
		}
		return report
	}
	before := inspect()
	for _, name := range []string{"hourly_credit_days", "hourly_account_preferences"} {
		found := false
		for _, table := range before.Tables {
			if table.Name == name && table.Rows == 1 {
				found = true
			}
		}
		if !found {
			t.Fatal("native time fixture was omitted from recovery evidence", name)
		}
	}
	if _, err = db.ExecContext(ctx, `SET TIME ZONE 'Asia/Tokyo'`); err != nil {
		t.Fatal(err)
	}
	if after := inspect(); after.Hash != before.Hash {
		t.Fatal("same database fingerprint changed with session time zone")
	}
	if _, err = db.ExecContext(ctx, `UPDATE hourly_account_preferences SET updated_at=updated_at+INTERVAL '1 microsecond' WHERE owner_id='native-time-owner'`); err != nil {
		t.Fatal(err)
	}
	if after := inspect(); after.Hash == before.Hash {
		t.Fatal("native timestamp modification was absent from recovery evidence")
	}
}
