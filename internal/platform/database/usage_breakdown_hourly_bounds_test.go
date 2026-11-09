package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestUsageBreakdownCombinedLimitConservesFractionalNodeAndHourlyOmissions(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	seconds := int64(7 * 86400)
	usageBreakdownPurchase(t, s, u, "current", 100, &seconds, at)
	for n := 0; n < 129; n++ {
		usageBreakdownRaw(t, s, u, "combo", "current", fmt.Sprintf("node-%03d", n), 1, 2, at)
	}
	for n, group := range []string{"hourly-a", "hourly-b"} {
		p := hourlyBreakdownEnrollment(t, s, u, group, group, at)
		hourlyBreakdownRaw(t, s, p, "shared", int64(3+n*2), int64(4+n*2), at)
		hourlyBreakdownReceipt(t, s, p, group+"-receipt", int64(11+n*2), at)
	}
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), at)
	if err != nil || len(value.Nodes) != 128 || len(value.HourlySquads) != 0 || !value.Coverage.Truncated || value.Coverage.Complete {
		t.Fatal(value, err)
	}
	if value.Omitted.UploadBytes != "9" || value.Omitted.DownloadBytes != "12" || value.Omitted.CostMinor != (model.RationalMinor{Numerator: "7324", Denominator: "301"}) {
		t.Fatal("combined omissions rounded or duplicated bytes/cost", value.Omitted)
	}
	if value.PositiveNodes.Traffic != "129" || value.PositiveNodes.Cost != "129" || value.PositiveEntries.Traffic != "131" || value.PositiveEntries.Cost != "131" || value.Cost.ActualMinor != "24" {
		t.Fatal("counts were capped or changed physical-only compatibility", value)
	}
	assertUsageBreakdownConservation(t, value)
}

func TestUsageBreakdownHourlyUsesRemainingCombinedSlotsInStableGroupOrder(t *testing.T) {
	s, u, at := usageBreakdownFixture(t)
	for n := 0; n < 127; n++ {
		usageBreakdownRaw(t, s, u, "combo", "historical", fmt.Sprintf("node-%03d", n), 1, 0, at)
	}
	for _, group := range []string{"z-group", "a-group"} {
		p := hourlyBreakdownEnrollment(t, s, u, group, group, at)
		hourlyBreakdownReceipt(t, s, p, group+"-receipt", 7, at)
	}
	value, err := s.XNodeUsageBreakdown(context.Background(), u.ID, at.Add(-72*time.Hour), at)
	if err != nil || len(value.Nodes)+len(value.HourlySquads) != model.MaxUsageBreakdownEntries || len(value.HourlySquads) != 1 || value.HourlySquads[0].GroupID != "a-group" || value.Omitted.CostMinor.Numerator != "7" {
		t.Fatal("combined slots were per-kind, unstable or lost idle costs", value, err)
	}
	if value.PositiveEntries.Traffic != "127" || value.PositiveEntries.Cost != "2" || !value.Coverage.Truncated {
		t.Fatal(value)
	}
	assertUsageBreakdownConservation(t, value)
}
