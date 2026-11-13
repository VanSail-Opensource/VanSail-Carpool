package database

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestAccessRepairRejectsChangedLifecycleAndClosure(t *testing.T) {
	for _, change := range []string{"purging", "purged", "generation", "closure"} {
		t.Run(change, func(t *testing.T) {
			f, ctx := newAccessRepairFixture(t, "closing"), context.Background()
			expected := f.period
			var err error
			switch change {
			case "purging", "purged":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE users SET account_status=$1 WHERE id=$2`, change, f.identity.UserID)
			case "generation":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, f.identity.UserID)
			case "closure":
				_, err = f.store.DB().ExecContext(ctx, `UPDATE xnode_periods SET closure_generation=closure_generation+1 WHERE id=$1`, f.period.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = f.store.RepairXNodeAccessPeriod(ctx, f.identity, f.purchase, expected, 0, f.now, false)
			if change == "closure" && !errors.Is(err, xnode.ErrConflict) || change != "closure" && !errors.Is(err, ErrAccountUnavailable) {
				t.Fatal("stale repair crossed lifecycle/closure authority", err)
			}
			period, err := f.store.XNodePeriod(ctx, expected.ID)
			if err != nil || period.Status != "closing" || period.UsedBytes != 77 {
				t.Fatal("rejected repair changed retained accounting", period, err)
			}
		})
	}
}

func TestAccessRepairKeepsRetiredParticipantEvidence(t *testing.T) {
	f, ctx := newAccessRepairFixture(t, "open"), context.Background()
	if err := f.store.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "repair-config", Name: "Repair fixture", DocumentCipher: "fixture"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"live", "retired"} {
		if err := f.store.SaveXNodeNode(ctx, xnode.Node{ID: id, Name: id, Enabled: true, ConfigID: "repair-config", SettingsCipher: "fixture", MultiplierFP: xnode.MultiplierScale}); err != nil {
			t.Fatal(err)
		}
		if err := f.store.TrackXNodePeriodNode(ctx, f.period.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_nodes SET enabled=0,removing_at=$1,retired_at=$1 WHERE id='retired'`, stamp(f.now)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.DB().ExecContext(ctx, `UPDATE xnode_period_nodes SET flushed_sequence=7,flushed_stream_id='captured-stream',flushed_legacy_sequence=3 WHERE period_id=$1`, f.period.ID); err != nil {
		t.Fatal(err)
	}
	period, err := f.store.BeginXNodePeriodClosure(ctx, f.period, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.RepairXNodeAccessPeriod(ctx, f.identity, f.purchase, period, 0, f.now, false); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"live", "retired"} {
		var sequence sql.NullInt64
		var stream string
		var legacy int64
		if err = f.store.DB().QueryRowContext(ctx, `SELECT flushed_sequence,flushed_stream_id,flushed_legacy_sequence FROM xnode_period_nodes WHERE period_id=$1 AND node_id=$2`, period.ID, id).Scan(&sequence, &stream, &legacy); err != nil {
			t.Fatal(err)
		}
		if id == "retired" && (!sequence.Valid || sequence.Int64 != 7 || stream != "captured-stream" || legacy != 3) {
			t.Fatal("permanent retirement evidence cleared", sequence, stream, legacy)
		}
		if id == "live" && (sequence.Valid || stream != "" || legacy != 0) {
			t.Fatal("live participant retained a previous closure barrier", sequence, stream, legacy)
		}
	}
}
