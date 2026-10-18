package connectivity

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestNativeTargetIDsAndPrivatePayloadIsolation(t *testing.T) {
	user := User{ID: 42, Username: "native-monitor", Status: "ACTIVE"}
	targets := []Target{{HostUUID: "published-tokyo", NativeOutbound: json.RawMessage(`{"protocol":"vless","settings":{"private":"secret"}}`)}}
	if err := validateSourceResult(42, user, targets); err != nil {
		t.Fatal("native endpoint ID rejected", err)
	}
	copy := cloneTargets(targets)
	copy[0].NativeOutbound[0] = '!'
	if targets[0].NativeOutbound[0] != '{' {
		t.Fatal("probe mutated its source credentials")
	}
	metadata := metadataTargets(targets)
	if len(metadata[0].NativeOutbound) > 0 || len(metadata[0].Resolved) > 0 {
		t.Fatal("credentials retained in metadata")
	}
	if err := validateSourceResult(42, user, append(targets, targets[0])); err == nil {
		t.Fatal("duplicate native endpoint accepted")
	}
	if err := validateSourceResult(42, user, []Target{{HostUUID: "published-tokyo", Resolved: json.RawMessage(`{}`)}}); err == nil {
		t.Fatal("legacy target lost its UUID identity guard")
	}
}

type uptimeProjectionRepository struct {
	Repository
	calls int
	items []Attempt
}

func (r *uptimeProjectionRepository) ConnectivityTimeline(context.Context, string, time.Time) ([]Attempt, error) {
	r.calls++
	return r.items, nil
}

func TestDisabledMonitoringDoesNotReadOrExposeHistoricStatus(t *testing.T) {
	settings, _, _, cfg := configuredDependencies(t)
	now := time.Now().UTC()
	observed := now.Add(-time.Second)
	repo := &uptimeProjectionRepository{items: []Attempt{{HostUUID: "published-tokyo", FinishedAt: &observed, Outcome: Outcome{Status: "connected"}}}}
	service := NewService(settings, nil, nil, repo, nil)
	subscription := MemberSubscription{ActiveCombo: true, Hosts: []SubscriptionHost{{UUID: "published-tokyo", Link: ptrString("vless://private")}}, Squads: []SubscriptionSquad{{UUID: "group", HostUUIDs: []string{"published-tokyo"}}}}
	service.ProjectMember(context.Background(), &subscription, now)
	if subscription.Summary.MonitoringEnabled || repo.calls != 0 || subscription.Summary.State != "unknown" || subscription.Hosts[0].Link == nil {
		t.Fatal("disabled monitoring exposed history or removed links", subscription, repo.calls)
	}
	cfg.ScheduledEnabled = true
	settings.set(t, cfg)
	service.ProjectMember(context.Background(), &subscription, now)
	if !subscription.Summary.MonitoringEnabled || repo.calls != 1 || subscription.Summary.State != "operational" {
		t.Fatal("enabled native monitoring did not project observations", subscription, repo.calls)
	}
}

func ptrString(value string) *string { return &value }
