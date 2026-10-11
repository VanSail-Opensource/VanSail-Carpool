package app

import (
	"context"
	"errors"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/admin"
	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/database"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
	"github.com/txyyddss/Remna-User-Panel/internal/providerops"
)

// ipLookupQueueTuning bounds dispatch rate and in-flight concurrency per source.
// Every queue is process-shared by all concurrent lookups, so the free public
// datasets (RIPEstat, CAIDA, Cloudflare Radar, BGPKIT, RouteViews, InternetDB)
// pace far below the keyed reputation providers to respect fair-use limits.
// Throughput is 1/MinInterval per source regardless of workers; workers only
// cap simultaneous in-flight requests.
type ipLookupQueueTuning struct {
	MinInterval time.Duration
	Workers     int
}

func ipLookupTuning(id string) ipLookupQueueTuning {
	switch id {
	case "checks":
		return ipLookupQueueTuning{MinInterval: 250 * time.Millisecond, Workers: 2}
	case "probes":
		return ipLookupQueueTuning{MinInterval: 100 * time.Millisecond, Workers: 4}
	case "proxy_lists", "dns":
		return ipLookupQueueTuning{MinInterval: 50 * time.Millisecond, Workers: 4}
	case "ipapi":
		return ipLookupQueueTuning{MinInterval: 250 * time.Millisecond, Workers: 4}
	case "ripe":
		return ipLookupQueueTuning{MinInterval: 600 * time.Millisecond, Workers: 2}
	case "caida":
		return ipLookupQueueTuning{MinInterval: 600 * time.Millisecond, Workers: 2}
	case iplookup.CloudflareID:
		// Radar enforces the tightest free-tier limits of the live sources.
		return ipLookupQueueTuning{MinInterval: 800 * time.Millisecond, Workers: 1}
	case "bgpkit", "routeviews", "shodan":
		return ipLookupQueueTuning{MinInterval: 500 * time.Millisecond, Workers: 1}
	case "peeringdb":
		return ipLookupQueueTuning{MinInterval: 3 * time.Second, Workers: 1}
	default:
		// Reputation providers execute serially with admin-configured quotas;
		// DNS PTR resolution is a cheap non-rate-limited lookup.
		return ipLookupQueueTuning{MinInterval: 250 * time.Millisecond, Workers: 1}
	}
}

func newIPLookupQueues() (map[string]*upstreamqueue.Queue, error) {
	result := map[string]*upstreamqueue.Queue{}
	for _, id := range iplookup.QueueIDs() {
		tuning := ipLookupTuning(id)
		queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "ip-lookup-" + id, Capacity: 32, MinInterval: tuning.MinInterval, Workers: tuning.Workers})
		if err != nil {
			return nil, err
		}
		result[id] = queue
	}
	return result, nil
}

func (q *providerQueues) startIPLookup(ctx context.Context) error {
	for _, id := range iplookup.QueueIDs() {
		if queue := q.iplookup[id]; queue != nil {
			if err := queue.Start(ctx); err != nil {
				_ = q.shutdownIPLookup(context.Background())
				return err
			}
		}
	}
	return nil
}

func (q *providerQueues) shutdownIPLookup(ctx context.Context) error {
	var result error
	for _, id := range iplookup.QueueIDs() {
		if queue := q.iplookup[id]; queue != nil {
			result = errors.Join(result, queue.Shutdown(ctx))
		}
	}
	return result
}

func newIPLookupService(store *database.Store, settings *admin.SettingsService, queues *providerQueues, dispatcher *providerops.Dispatcher, key []byte) (*iplookup.Service, *iplookup.Worker, *iplookup.LiveService, error) {
	providers := []iplookup.Provider{}
	for _, descriptor := range iplookup.Registry {
		providers = append(providers, iplookup.NewHTTPProvider(descriptor.ID, queues.iplookup[descriptor.ID], settings))
	}
	live := iplookup.NewLiveService(settings, queues.iplookup, store)
	worker := iplookup.NewWorker(store, providers)
	worker.Live = live
	worker.Enhanced = NewIPLookupEnhanced(settings, queues.iplookup, live)
	if err := dispatcher.Register(iplookup.OperationKind, worker); err != nil {
		return nil, nil, nil, err
	}
	service := iplookup.NewService(store, key)
	service.Live = live
	return service, worker, live, nil
}
