package connectivity

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestManualCheckDeduplicatesAndPersistsBeforeNetwork(t *testing.T) {
	settings, source, repository, _ := configuredDependencies(t)
	started, release := make(chan struct{}), make(chan struct{})
	service := startTestService(t, settings, source, testProbe(func(ctx context.Context, _ Target, _ Config) Outcome {
		close(started)
		select {
		case <-ctx.Done():
			return Outcome{Status: "interrupted"}
		case <-release:
			return Outcome{Status: "connected"}
		}
	}), repository)
	caller, cancelCaller := context.WithCancel(context.Background())
	first, err := service.Start(caller, "manual")
	if err != nil {
		t.Fatal(err)
	}
	cancelCaller()
	waitSignal(t, started)
	items := repository.attempts()
	if len(items) != 1 || items[0].Status != "running" || items[0].RunID != first.ID {
		t.Fatalf("probe ran before retained admission: %+v", items)
	}
	duplicate, err := service.Start(context.Background(), "manual")
	if err != nil || duplicate.ID != first.ID {
		t.Fatalf("duplicate manual run was not deduplicated: %+v %v", duplicate, err)
	}
	close(release)
	waitFor(t, func() bool { return repository.attempts()[0].Status == "connected" })
	snapshot, err := service.Snapshot(context.Background())
	if err != nil || len(snapshot.Hosts) != 1 || snapshot.Hosts[0].Latest == nil {
		t.Fatalf("retained latest projection missing: %+v %v", snapshot, err)
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil || strings.Contains(string(encoded), "private-secret") {
		t.Fatalf("snapshot contained resolved credentials: %s %v", encoded, err)
	}
}

func TestPersistenceFailurePreventsNetworkProbe(t *testing.T) {
	settings, source, repository, _ := configuredDependencies(t)
	repository.beginErr = errors.New("storage unavailable")
	called := make(chan struct{}, 1)
	service := startTestService(t, settings, source, testProbe(func(context.Context, Target, Config) Outcome {
		called <- struct{}{}
		return Outcome{Status: "connected"}
	}), repository)
	if _, err := service.Start(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		service.mu.RLock()
		defer service.mu.RUnlock()
		return service.last != nil
	})
	select {
	case <-called:
		t.Fatal("probe ran without a persisted attempt")
	default:
	}
	service.mu.RLock()
	defer service.mu.RUnlock()
	if service.last.ErrorCode != "CONNECTIVITY_STORAGE_UNAVAILABLE" {
		t.Fatalf("unexpected storage failure: %+v", service.last)
	}
}

func TestConfiguredSetupFailureIsRetained(t *testing.T) {
	settings, source, repository, cfg := configuredDependencies(t)
	source.fail(&CodeError{Code: "CONNECTIVITY_SUBSCRIPTION_UNAVAILABLE"})
	service := startTestService(t, settings, source, testProbe(func(context.Context, Target, Config) Outcome {
		t.Error("source failure reached network probe")
		return Outcome{Status: "connected"}
	}), repository)
	if _, err := service.Start(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { items := repository.attempts(); return len(items) == 1 && items[0].FinishedAt != nil })
	item := repository.attempts()[0]
	if item.HostUUID != "" || item.ProxyUserID != cfg.ProxyUserID || item.Status != "error" ||
		item.ErrorCode != "CONNECTIVITY_SUBSCRIPTION_UNAVAILABLE" {
		t.Fatalf("setup failure metadata lost: %+v", item)
	}
}

func TestUnconfiguredManualCheckHasNoAttempt(t *testing.T) {
	settings, source, repository, _ := configuredDependencies(t)
	settings.set(t, DefaultConfig())
	service := startTestService(t, settings, source, testProbe(func(context.Context, Target, Config) Outcome {
		t.Error("unconfigured check reached probe")
		return Outcome{Status: "error"}
	}), repository)
	if _, err := service.Start(context.Background(), "manual"); ErrorCode(err) != "CONNECTIVITY_NOT_CONFIGURED" {
		t.Fatalf("unexpected unconfigured response: %v", err)
	}
	if len(repository.attempts()) != 0 {
		t.Fatal("unconfigured request retained an invalid account ID")
	}
}
