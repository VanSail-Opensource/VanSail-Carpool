package xnode

import (
	"context"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type profileMetricRepository struct {
	Repository
	phase string
	date  string
}

func (r profileMetricRepository) identity() Identity {
	return Identity{ProxyUserID: 1, UserID: "member", CurrentPeriodID: "period"}
}
func (r profileMetricRepository) XNodeIdentityByUser(context.Context, string) (Identity, error) {
	return r.identity(), nil
}
func (r profileMetricRepository) XNodeIdentity(context.Context, int64) (Identity, error) {
	return r.identity(), nil
}
func (r profileMetricRepository) XNodeIdentities(context.Context) ([]Identity, error) {
	return []Identity{r.identity()}, nil
}
func (r profileMetricRepository) XNodeNodes(context.Context) ([]Node, error) {
	return []Node{{ID: "node", Name: "Tokyo", Enabled: r.phase != "disabled"}}, nil
}
func (r profileMetricRepository) XNodeNode(context.Context, string) (Node, error) {
	return Node{ID: "node", Name: "Tokyo", Enabled: r.phase != "disabled"}, nil
}
func (r profileMetricRepository) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return &model.Purchase{ID: "purchase", SquadUUIDs: []string{"group"}}, nil
}
func (r profileMetricRepository) XNodeUserRestricted(context.Context, string) (bool, error) {
	return false, nil
}
func (r profileMetricRepository) UserDisabledSquads(context.Context, string) ([]string, error) {
	return nil, nil
}
func (r profileMetricRepository) XNodeGroups(context.Context) ([]Group, error) {
	return []Group{{ID: "group", Enabled: true, Bindings: []GroupBinding{{NodeID: "node"}}}}, nil
}
func (r profileMetricRepository) XNodePeriod(context.Context, string) (Period, error) {
	return Period{ID: "period", PurchaseID: "purchase", UsedBytes: 9007199254741048, LimitBytes: math.MaxInt64, Status: "open"}, nil
}
func (r profileMetricRepository) XNodePeriods(context.Context, int64) ([]Period, error) {
	value, _ := r.XNodePeriod(context.Background(), "period")
	return []Period{value}, nil
}
func (r profileMetricRepository) XNodeImportedUsage(context.Context, int64) (ImportedUsage, error) {
	return ImportedUsage{LifetimeBytes: 1000}, nil
}
func (r profileMetricRepository) XNodeUsage(context.Context, int64, time.Time, time.Time) ([]DailyUsage, error) {
	return []DailyUsage{{NodeID: "node", Date: r.date, UploadBytes: 9007199254740993, DownloadBytes: 10, WeightedBytes: 9007199254741003}}, nil
}
func (r profileMetricRepository) XNodeImportedHistory(context.Context, int64, time.Time, time.Time) ([]ImportedHistory, error) {
	return []ImportedHistory{{NodeID: "node", Date: r.date, RawBytes: 30, WeightedBytes: 45}}, nil
}

func profileMetricMaster(t *testing.T, phase string) (*Master, time.Time) {
	t.Helper()
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	service := NewService(profileMetricRepository{phase: phase, date: now.Format(time.DateOnly)}, cacheTestSettings{}, nil, nil, nil)
	service.now = func() time.Time { return now }
	master := NewMaster(service, slog.New(slog.NewTextHandler(io.Discard, nil)))
	master.peers["node"] = &agentPeer{}
	master.live["node"] = liveSnapshot{receivedAt: now, hello: wire.AgentHello{Capabilities: []string{"sampled_activity_v1"}}, report: wire.TelemetryReport{Ready: true, Synchronized: true, Time: now, Users: wire.UserMetrics{Connections: map[int]int{1: 2}, Rates: map[int]wire.UserRate{1: {UploadBytesPerSecond: 100, DownloadBytesPerSecond: 200}}}}}
	return master, now
}

func TestProfileActivityUnavailableAfterDisableDisconnectOrStaleness(t *testing.T) {
	for _, mode := range []string{"native", "disabled", "disconnected", "stale"} {
		t.Run(mode, func(t *testing.T) {
			phase := "native"
			if mode == "disabled" {
				phase = "disabled"
			}
			master, now := profileMetricMaster(t, phase)
			if mode == "disconnected" {
				delete(master.peers, "node")
			}
			if mode == "stale" {
				snapshot := master.live["node"]
				snapshot.receivedAt = now.Add(-time.Minute)
				master.live["node"] = snapshot
			}
			values, err := master.UserNodeMetrics(context.Background(), "member")
			if err != nil || len(values) != 1 || values[0].Available != (mode == "native") {
				t.Fatal(mode, values, err)
			}
			if mode != "native" && (values[0].Connections != "0" || values[0].CurrentUse || values[0].UploadBytesPerSecond != 0) {
				t.Fatal("unavailable activity leaked", values)
			}
			if mode == "disabled" {
				transport, err := master.Metrics(context.Background())
				if err != nil || transport.OnlineNodes != 1 || transport.Nodes[0].Available || transport.ActiveConnections != 0 {
					t.Fatal("transport liveness was coupled to ownership", transport, err)
				}
			}
		})
	}
}

func TestProfileSevenDayAccountingPreservesExactCountersAndLegacyDirection(t *testing.T) {
	master, _ := profileMetricMaster(t, "native")
	value, err := master.ProfileMetrics(context.Background(), "member")
	if err != nil || len(value.Usage) != 7 {
		t.Fatal(value, err)
	}
	day := value.Usage[6]
	if day.UploadBytes != "9007199254740993" || day.DownloadBytes != "10" || day.LegacyBytes != "30" || day.WeightedBytes != "9007199254741048" {
		t.Fatal("accounting rounded, duplicated, or invented a source split", day)
	}
	if value.UsedTrafficBytes != "9007199254741048" || value.LifetimeTrafficBytes != "9007199254742003" {
		t.Fatal("period/lifetime projection diverged", value)
	}
}
