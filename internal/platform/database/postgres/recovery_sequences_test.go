package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/testdatabase"
)

func TestRecoveryFingerprintRetainsAllApplicationSequenceStates(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db, err := testdatabase.Open(ctx, t)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	inspect := func() postgres.Report {
		t.Helper()
		report, err := postgres.InspectRecovery(ctx, db, make([]byte, 32))
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	before := inspect()
	for _, name := range []string{
		"outbox_jobs_record_order_seq", "provider_operations_record_order_seq",
		"ip_lookup_reports_record_order_seq", "ip_lookup_live_snapshots_record_order_seq",
		"pricing_features_feature_key_seq", "pricing_forecast_entities_entity_key_seq", "pricing_forecast_batches_batch_key_seq",
		"xnode_identities_proxy_user_id_seq", "xnode_firewall_members_ordinal_seq", "xnode_service_proxy_id_seq",
	} {
		if _, exists := before.SequenceStates[name]; !exists {
			t.Fatal("application sequence omitted from recovery evidence", name)
		}
	}
	for _, state := range []struct {
		value  int64
		called bool
	}{{41, true}, {41, false}, {42, false}} {
		if _, err := db.ExecContext(ctx, `SELECT setval('outbox_jobs_record_order_seq',$1,$2)`, state.value, state.called); err != nil {
			t.Fatal(err)
		}
		after := inspect()
		got := after.SequenceStates["outbox_jobs_record_order_seq"]
		if got.LastValue != state.value || got.IsCalled != state.called || after.Hash == before.Hash {
			t.Fatal("sequence-only recovery damage was invisible", got)
		}
		for i := range before.Tables {
			if before.Tables[i] != after.Tables[i] {
				t.Fatal("sequence-only fixture unexpectedly changed authoritative rows")
			}
		}
		before = after
	}
	if _, err := db.ExecContext(ctx, `ALTER SEQUENCE outbox_jobs_record_order_seq INCREMENT BY 2`); err != nil {
		t.Fatal(err)
	}
	after := inspect()
	if after.Hash == before.Hash || after.SequenceStates["outbox_jobs_record_order_seq"].Increment != 2 {
		t.Fatal("changed next-value behavior was absent from recovery evidence")
	}
}
