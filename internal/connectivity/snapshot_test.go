package connectivity

import (
	"context"
	"testing"
	"time"
)

func TestSourceFailurePreservesRetainedLatestAndSafeMetadata(t *testing.T) {
	settings, source, repository, _ := configuredDependencies(t)
	service := startTestService(t, settings, source, testProbe(func(context.Context, Target, Config) Outcome {
		return Outcome{Status: "connected"}
	}), repository)
	if _, err := service.Start(context.Background(), "manual"); err != nil {
		t.Fatal(err)
	}
	waitCompletedRun(t, service, 1, repository)
	service.mu.Lock()
	if len(service.targets) != 1 || len(service.targets[0].Resolved) != 0 {
		service.mu.Unlock()
		t.Fatal("metadata cache retained proxy credentials")
	}
	service.loadedAt = time.Time{}
	service.mu.Unlock()
	source.fail(&CodeError{Code: "CONNECTIVITY_SUBSCRIPTION_UNAVAILABLE"})
	snapshot, err := service.Snapshot(context.Background())
	if err != nil || !snapshot.Stale || snapshot.ErrorCode != "CONNECTIVITY_SUBSCRIPTION_UNAVAILABLE" ||
		len(snapshot.Hosts) != 1 || snapshot.Hosts[0].Latest == nil || snapshot.Hosts[0].Latest.Status != "connected" ||
		snapshot.Hosts[0].Remark != "test" {
		t.Fatalf("source failure erased the previous result: %+v %v", snapshot, err)
	}
	source.mu.Lock()
	loads := source.loads
	source.mu.Unlock()
	if _, err := service.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.loads != loads {
		t.Fatal("polling retried a cached source failure immediately")
	}
}

func TestRestartSourceOutageStillShowsRetainedResults(t *testing.T) {
	settings, source, repository, cfg := configuredDependencies(t)
	started := time.Now().UTC().Add(-time.Minute)
	finished := started.Add(time.Second)
	repository.items = []Attempt{{ID: "retained", HostUUID: source.targets[0].HostUUID, ConfigHash: ConfigHash(cfg),
		ProxyUserID: cfg.ProxyUserID, StartedAt: started, FinishedAt: &finished,
		Outcome: Outcome{Status: "connected"}}}
	source.fail(&CodeError{Code: "CONNECTIVITY_ACCOUNT_UNAVAILABLE"})
	service := startTestService(t, settings, source, testProbe(func(context.Context, Target, Config) Outcome {
		t.Error("restart outage reached network probe")
		return Outcome{Status: "error"}
	}), repository)
	snapshot, err := service.Snapshot(context.Background())
	if err != nil || !snapshot.Stale || len(snapshot.Hosts) != 1 || snapshot.Hosts[0].Latest == nil ||
		snapshot.Hosts[0].Latest.ID != "retained" || snapshot.Hosts[0].Remark != "" {
		t.Fatalf("restart lost retained results or invented metadata: %+v %v", snapshot, err)
	}
}
