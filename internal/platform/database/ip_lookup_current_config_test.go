package database

import (
	"context"

	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func TestIPLookupResumeAndRepairLoadCurrentSettings(t *testing.T) {
	ctx := context.Background()
	store, service, user, _, _ := ipLookupFixture(t, 2)
	op := submitIP(t, service, user, "current-settings", false)
	r, c, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	c.Retry = &iplookup.RetryPolicy{Enabled: true, MaxAttempts: 2, DelaySeconds: 321}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.DB().ExecContext(ctx, `UPDATE settings SET value=$1 WHERE key=$2`, string(raw), iplookup.SettingKey); err != nil {
		t.Fatal(err)
	}
	_, resumed, err := store.IPLookupRun(ctx, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, repaired, err := store.IPLookupReportByID(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Retry.DelaySeconds != 321 || repaired.Retry.DelaySeconds != 321 {
		t.Fatal("resume or repair used frozen config")
	}
}
