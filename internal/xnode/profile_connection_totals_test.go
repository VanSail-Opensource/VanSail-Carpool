package xnode

import (
	"context"
	"math"
	"testing"
)

type profileSecondaryPrincipalRepository struct {
	profileMetricRepository
	hourlyPrincipalRepository
}

func (profileSecondaryPrincipalRepository) XNodeHourlyPrincipals(context.Context, string, string) ([]HourlyPrincipal, error) {
	return []HourlyPrincipal{{ProxyUserID: 2, Status: "ended"}}, nil
}

func (profileSecondaryPrincipalRepository) XNodeEndpoints(context.Context) ([]Endpoint, error) {
	return nil, nil
}

func TestProfileNodeConnectionsRemainExactAcrossOwnedPrincipals(t *testing.T) {
	master, _ := profileMetricMaster(t, "native")
	master.service.repo = profileSecondaryPrincipalRepository{}
	snapshot := master.live["node"]
	snapshot.report.Users.Connections = map[int]int{1: math.MaxInt64, 2: math.MaxInt64}
	snapshot.report.Users.Rates = nil
	if !validMetrics(snapshot.report.Users) {
		t.Fatal("fixture must contain valid per-principal telemetry")
	}
	master.live["node"] = snapshot
	items, err := master.UserNodeMetrics(context.Background(), "member")
	if err != nil || len(items) != 1 || items[0].Connections != "18446744073709551614" || !items[0].CurrentUse {
		t.Fatal("owned-principal totals overflowed or lost current activity", items, err)
	}
}
