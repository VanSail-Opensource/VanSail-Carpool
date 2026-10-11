package iplookup

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

type failingResolver struct{}

func (failingResolver) LookupAddr(context.Context, string) ([]string, error) {
	return nil, errors.New("resolver unavailable")
}

type liveRetryRepo struct {
	details  LiveDetails
	status   string
	gone     bool
	conflict bool
	updates  int
	enqueued []enqueuedRetry
}

func (r *liveRetryRepo) CachedIPDetails(context.Context, string) (*LiveDetails, error) {
	return nil, nil
}

func (r *liveRetryRepo) SaveIPDetails(_ context.Context, _ string, details LiveDetails, _ time.Time) (string, error) {
	r.details = details
	return "snapshot", nil
}

func (r *liveRetryRepo) IPLiveSnapshotByID(context.Context, string) (LiveDetails, string, error) {
	if r.gone {
		return LiveDetails{}, "", ErrRepairGone
	}
	return r.details, r.status, nil
}

func (r *liveRetryRepo) UpdateIPLiveSnapshot(_ context.Context, _ string, details LiveDetails) error {
	if r.conflict {
		return ErrRepairConflict
	}
	r.details, r.updates = details, r.updates+1
	return nil
}

func (r *liveRetryRepo) EnqueueOutbox(_ context.Context, kind, payload string, at time.Time) error {
	r.enqueued = append(r.enqueued, enqueuedRetry{kind: kind, payload: payload, at: at})
	return nil
}

func healthyMeta() LiveMeta { return LiveMeta{Status: "success", Source: "fixture"} }

// partiallyFailingPTR leaves only the resolver section failed.
func partiallyFailingPTR() LiveDetails {
	return LiveDetails{
		SnapshotID: "snapshot", IP: "1.1.1.1", SourceIP: "1.1.1.1", FetchedAt: "2026-10-11T00:00:00Z",
		Topology: BGPTopology{ProjectionVersion: 2, LiveMeta: healthyMeta(), RelationshipMeta: healthyMeta(), ClassificationMeta: healthyMeta(), Prefix: "1.1.1.0/24", Origins: []uint32{13335}},
		Scores:   NetworkScores{LiveMeta: healthyMeta()},
		Block:    AbuseBlock{LiveMeta: healthyMeta()},
		Shodan:   ShodanDetails{LiveMeta: healthyMeta()},
		PTR:      PTRDetails{LiveMeta: LiveMeta{Status: "error", Source: "dns", ErrorCode: "IP_DETAILS_UNAVAILABLE"}},
	}
}

func TestLiveRetryPlanCascadesTopologyFailures(t *testing.T) {
	t.Parallel()
	independent := partiallyFailingPTR()
	independent.PTR.LiveMeta = healthyMeta()
	independent.Block.LiveMeta = LiveMeta{Status: "error"}
	independent.Shodan.LiveMeta = LiveMeta{Status: "partial"}
	if plan := liveRetryPlanOf(independent); !plan.Block || !plan.Shodan || plan.Topology || plan.Graph || plan.ASNs || plan.PTR || plan.Scores {
		t.Fatalf("independent failures misplanned: %+v", plan)
	}

	cascading := partiallyFailingPTR()
	cascading.PTR.LiveMeta = healthyMeta()
	cascading.Topology.LiveMeta = LiveMeta{Status: "error"}
	if plan := liveRetryPlanOf(cascading); !plan.Topology || !plan.Graph || plan.Block || !plan.ASNs || plan.PTR || plan.Scores || plan.Shodan {
		t.Fatalf("topology cascade lost: %+v", plan)
	}

	asnFailed := partiallyFailingPTR()
	asnFailed.PTR.LiveMeta = healthyMeta()
	asnFailed.ASNs = []ASNDetails{{ASN: 13335, Sources: map[string]LiveMeta{"bgpkit": {Status: "error"}}}}
	if plan := liveRetryPlanOf(asnFailed); !plan.ASNs || plan.Topology || plan.Graph {
		t.Fatalf("asn failure not planned: %+v", plan)
	}

	healthy := partiallyFailingPTR()
	healthy.PTR.LiveMeta = healthyMeta()
	if plan := liveRetryPlanOf(healthy); !plan.empty() {
		t.Fatalf("healthy snapshot planned repair: %+v", plan)
	}
}

func retryLiveService(t *testing.T, resolver interface {
	LookupAddr(context.Context, string) ([]string, error)
}, repository *liveRetryRepo) *LiveService {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "dns", Capacity: 4})
	if err != nil {
		t.Fatal(err)
	}
	if err := queue.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Shutdown(context.Background()) })
	settings := liveConfigSettings{SettingKey: `{"enabled":true,"lookupFeeTxb":"0","refreshFeeTxb":"0","providers":[{"id":"abuseipdb","enabled":true}]}`}
	s := NewLiveService(settings, map[string]*upstreamqueue.Queue{"dns": queue}, repository)
	s.resolver = resolver
	return s
}

func liveRetryJob(snapshotID string, attempt int) model.OutboxJob {
	payload := fmt.Sprintf(`{"snapshotId":%q,"attempt":%d}`, snapshotID, attempt)
	return model.OutboxJob{Payload: payload}
}

func TestLiveRetryRepairsPTRSectionAndStopsWhenHealthy(t *testing.T) {
	t.Parallel()
	repo := &liveRetryRepo{details: partiallyFailingPTR(), status: "partial"}
	s := retryLiveService(t, liveResolver{}, repo)
	if err := s.HandleLiveRetryOutbox(context.Background(), liveRetryJob("snapshot", 1)); err != nil {
		t.Fatal(err)
	}
	if repo.updates != 1 || LivePartial(repo.details) || len(repo.enqueued) != 0 {
		t.Fatalf("ptr repair incomplete: updates=%d partial=%v enqueued=%d", repo.updates, LivePartial(repo.details), len(repo.enqueued))
	}
}

func TestLiveRetryChainsWhileSectionsKeepFailing(t *testing.T) {
	t.Parallel()
	repo := &liveRetryRepo{details: partiallyFailingPTR(), status: "partial"}
	s := retryLiveService(t, failingResolver{}, repo)
	fixed := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	if err := s.HandleLiveRetryOutbox(context.Background(), liveRetryJob("snapshot", 1)); err != nil {
		t.Fatal(err)
	}
	if repo.updates != 1 || len(repo.enqueued) != 1 {
		t.Fatalf("chain not scheduled: updates=%d enqueued=%d", repo.updates, len(repo.enqueued))
	}
	next := repo.enqueued[0]
	if next.kind != LiveRetryOutboxKind || next.at != fixed.Add(time.Minute) || next.payload != `{"snapshotId":"snapshot","attempt":2}` {
		t.Fatalf("wrong chained job: %+v %s", next, next.payload)
	}
	if !LivePartial(repo.details) {
		t.Fatal("failed section vanished without evidence")
	}
}

func TestLiveRetrySkipsDisabledPolicyGoneRowsAndHealthyStatuses(t *testing.T) {
	t.Parallel()
	repo := &liveRetryRepo{details: partiallyFailingPTR(), status: "partial"}
	s := retryLiveService(t, liveResolver{}, repo)

	repo.gone = true
	if err := s.HandleLiveRetryOutbox(context.Background(), liveRetryJob("snapshot", 1)); err != nil {
		t.Fatal(err)
	}
	if repo.updates != 0 {
		t.Fatal("gone snapshot repaired")
	}
	repo.gone = false

	repo.status = "succeeded"
	if err := s.HandleLiveRetryOutbox(context.Background(), liveRetryJob("snapshot", 1)); err != nil {
		t.Fatal(err)
	}
	if repo.updates != 0 {
		t.Fatal("healthy snapshot repaired")
	}
	repo.status = "partial"

	if err := s.HandleLiveRetryOutbox(context.Background(), liveRetryJob("snapshot", 4)); err != nil {
		t.Fatal(err)
	}
	if repo.updates != 0 {
		t.Fatal("exhausted policy repaired a section")
	}

	repo.conflict = true
	if err := s.HandleLiveRetryOutbox(context.Background(), liveRetryJob("snapshot", 1)); err != nil {
		t.Fatal(err)
	}
	if len(repo.enqueued) != 0 {
		t.Fatal("conflicting snapshot chained")
	}
}
