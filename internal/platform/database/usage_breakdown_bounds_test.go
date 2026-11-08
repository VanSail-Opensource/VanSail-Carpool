package database

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestUsageBreakdownAggregatesBeforeNodeLimitAndConservesOmittedValue(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	seconds := int64(7 * 86400)
	usageBreakdownPurchase(t, s, u, "current", 100, &seconds, at)
	for n := 0; n < 130; n++ {
		id := fmt.Sprintf("node-%03d", n)
		usageBreakdownRaw(t, s, u, "combo", "current", id, 1, 2, at)
		// Multiple references on the same node must not spend its response slot.
		usageBreakdownRaw(t, s, u, "hourly", "other", id, 3, 4, at)
	}
	usageBreakdownExec(t, s, `INSERT INTO xnode_usage_history_state(singleton,started_at) VALUES(true,$1)`, at.Add(-73*time.Hour))
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), at)
	if err != nil || len(value.Nodes) != 128 || !value.Coverage.Truncated || value.Coverage.Complete {
		t.Fatal(value, err)
	}
	if value.Nodes[0].NodeID != "node-000" || value.Nodes[127].NodeID != "node-127" || value.Omitted.UploadBytes != "8" || value.Omitted.DownloadBytes != "12" {
		t.Fatal("aggregation/limit lost stable node identities", value.Omitted)
	}
	if value.PositiveNodes.Traffic != "130" || value.PositiveNodes.Cost != "130" {
		t.Fatal("positive nodes counted after cap/grouping", value.PositiveNodes)
	}
	if value.Omitted.CostMinor != (model.RationalMinor{Numerator: "60", Denominator: "91"}) {
		t.Fatal("omitted estimate rounded", value.Omitted)
	}
	assertUsageBreakdownConservation(t, value)
}

func TestUsageBreakdownEmptyReadDoesNotInventFloorAndRejectsClosedOwners(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	ctx := context.Background()
	value, err := s.XNodeUsageBreakdown(ctx, u.ID, at.Add(-72*time.Hour), at)
	if err != nil || value.Nodes == nil || len(value.Nodes) != 0 || value.Coverage.Since != nil || value.Coverage.Complete || value.Cost.Legacy.Status != model.UsageLegacyNone {
		t.Fatal(value, err)
	}
	var count int
	if err = s.DB().QueryRow(`SELECT COUNT(*) FROM xnode_usage_history_state`).Scan(&count); err != nil || count != 0 {
		t.Fatal("GET initialized coverage", count, err)
	}
	for _, owner := range []string{"", "missing"} {
		_, err = s.XNodeUsageBreakdown(ctx, owner, at.Add(-time.Hour), at)
		want := error(ErrNotFound)
		if owner == "" {
			want = xnode.ErrInvalid
		}
		if !errors.Is(err, want) {
			t.Fatal(owner, err)
		}
	}
	for _, span := range []time.Duration{-time.Second, 73 * time.Hour} {
		_, err = s.XNodeUsageBreakdown(ctx, u.ID, at, at.Add(span))
		if !errors.Is(err, xnode.ErrInvalid) {
			t.Fatal("invalid window accepted", span, err)
		}
	}
	usageBreakdownExec(t, s, `UPDATE users SET account_status='purging' WHERE id=$1`, u.ID)
	if _, err = s.XNodeUsageBreakdown(ctx, u.ID, at.Add(-time.Hour), at); !errors.Is(err, ErrAccountUnavailable) {
		t.Fatal("fenced owner history exposed", err)
	}
}
