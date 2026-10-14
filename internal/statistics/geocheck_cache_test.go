package statistics

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const cachedNodeUUID = "373f14bc-089a-4c3a-91c3-3421e7c83367"

func TestGeocheckCacheDueReuseRetryAndPruning(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	cache := geocheckCache{}
	nodes := []Node{{UUID: cachedNodeUUID}, {UUID: "removed"}}
	due := cache.markDue(nodes, now)
	if len(due) != 2 {
		t.Fatalf("initial due = %v", due)
	}
	cache.finish(cachedNodeUUID, validCachedImage(now), true)
	cache.finish("removed", model.StatisticsNodeGeocheck{}, false)
	if due = cache.markDue(nodes, now.Add(time.Minute)); len(due) != 0 {
		t.Fatalf("fresh image due = %v", due)
	}
	if due = cache.markDue(nodes, now.Add(geocheckRetryInterval)); len(due) != 1 || due[0] != "removed" {
		t.Fatalf("failed retry due = %v", due)
	}
	if due = cache.markDue(nodes, now.Add(geocheckTTL)); len(due) != 1 || due[0] != cachedNodeUUID {
		t.Fatalf("expired image due = %v", due)
	}
	cache.markDue([]Node{{UUID: cachedNodeUUID}}, now.Add(geocheckTTL))
	if _, ok := cache.get("removed"); ok {
		t.Fatal("removed node remains cached")
	}
}

func TestGeocheckWorkerStoresOnlyValidatedCompletedResults(t *testing.T) {
	t.Parallel()
	provider := &geocheckProvider{result: GeocheckResult{Completed: true, Success: true, NodeUUID: cachedNodeUUID, Image: &GeocheckImage{
		Format: "svg", MediaType: "image/svg+xml", Encoding: "base64", Data: "PHN2Zy8+",
	}}}
	service := &Service{provider: provider}
	service.geocheck.markDue([]Node{{UUID: cachedNodeUUID}}, time.Now().UTC())
	service.runGeocheck(context.Background(), cachedNodeUUID)
	if result, ok := service.NodeGeocheck(cachedNodeUUID); !ok || result.Image.Data != "PHN2Zy8+" {
		t.Fatalf("NodeGeocheck() = %#v, %v", result, ok)
	}
	provider.result.NodeUUID = "different"
	service.geocheck.markDue([]Node{{UUID: cachedNodeUUID}}, time.Now().Add(geocheckTTL))
	service.runGeocheck(context.Background(), cachedNodeUUID)
	if result, ok := service.NodeGeocheck(cachedNodeUUID); !ok || result.Image.Data != "PHN2Zy8+" {
		t.Fatalf("invalid refresh replaced last success: %#v, %v", result, ok)
	}
}

func TestValidGeocheckImageRejectsInvalidResultFields(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	valid := GeocheckResult{Completed: true, Success: true, NodeUUID: cachedNodeUUID, Image: &GeocheckImage{
		Format: "svg", MediaType: "image/svg+xml", Encoding: "base64", Data: "PHN2Zy8+",
	}}
	cases := []struct {
		name   string
		result GeocheckResult
	}{
		{name: "missing image", result: GeocheckResult{Completed: true, Success: true, NodeUUID: cachedNodeUUID}},
		{name: "different node", result: GeocheckResult{Completed: true, Success: true, NodeUUID: "different", Image: valid.Image}},
		{name: "wrong format", result: GeocheckResult{Completed: true, Success: true, NodeUUID: cachedNodeUUID, Image: &GeocheckImage{Format: "png", MediaType: "image/svg+xml", Encoding: "base64", Data: "PHN2Zy8+"}}},
		{name: "wrong media type", result: GeocheckResult{Completed: true, Success: true, NodeUUID: cachedNodeUUID, Image: &GeocheckImage{Format: "svg", MediaType: "image/png", Encoding: "base64", Data: "PHN2Zy8+"}}},
		{name: "wrong encoding", result: GeocheckResult{Completed: true, Success: true, NodeUUID: cachedNodeUUID, Image: &GeocheckImage{Format: "svg", MediaType: "image/svg+xml", Encoding: "utf8", Data: "PHN2Zy8+"}}},
		{name: "invalid payload", result: GeocheckResult{Completed: true, Success: true, NodeUUID: cachedNodeUUID, Image: &GeocheckImage{Format: "svg", MediaType: "image/svg+xml", Encoding: "base64", Data: "not base64"}}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if _, ok := validGeocheckImage(cachedNodeUUID, test.result, now); ok {
				t.Fatal("validGeocheckImage() accepted an invalid result")
			}
		})
	}
}

func TestGeocheckWorkersAreBoundedAndCancellationClearsPendingWork(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, geocheckWorkers)
	release := make(chan struct{})
	provider := &geocheckProvider{request: func(ctx context.Context, _ string) (string, error) {
		started <- struct{}{}
		select {
		case <-release:
			return "job", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}}
	service := &Service{provider: provider}
	nodes := make([]Node, geocheckWorkers+1)
	for index := range nodes {
		nodes[index].UUID = cachedNodeUUID + string(rune('a'+index))
	}
	due := service.geocheck.markDue(nodes, time.Now().UTC())
	done := make(chan struct{})
	go func() { service.runGeochecks(context.Background(), due); close(done) }()
	for range geocheckWorkers {
		<-started
	}
	if got := provider.active.Load(); got > geocheckWorkers {
		t.Fatalf("active workers = %d", got)
	}
	close(release)
	<-done

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.geocheck.markDue(nodes, time.Now().Add(geocheckTTL))
	service.runGeochecks(ctx, due)
	for nodeUUID, entry := range service.geocheck.entries {
		if entry.inProgress {
			t.Fatalf("cancelled node %q remains in progress", nodeUUID)
		}
	}
}

func validCachedImage(checkedAt time.Time) model.StatisticsNodeGeocheck {
	return model.StatisticsNodeGeocheck{NodeUUID: cachedNodeUUID, CheckedAt: checkedAt, Image: model.StatisticsNodeGeocheckImage{Data: "PHN2Zy8+"}}
}

type geocheckProvider struct {
	statisticsProviderStub
	result  GeocheckResult
	request func(context.Context, string) (string, error)
	active  atomic.Int32
	mu      sync.Mutex
}

func (p *geocheckProvider) RequestNodeGeocheck(ctx context.Context, nodeUUID string) (string, error) {
	p.active.Add(1)
	defer p.active.Add(-1)
	if p.request != nil {
		return p.request(ctx, nodeUUID)
	}
	return "job", nil
}
func (p *geocheckProvider) NodeGeocheckResult(context.Context, string) (GeocheckResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.result.Completed {
		return p.result, nil
	}
	return GeocheckResult{}, errors.New("result is unavailable")
}
