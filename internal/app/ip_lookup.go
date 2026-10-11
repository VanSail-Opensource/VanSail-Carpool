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

func newIPLookupQueues() (map[string]*upstreamqueue.Queue, error) {
	result := map[string]*upstreamqueue.Queue{}
	for _, id := range iplookup.QueueIDs() {
		workers := 1
		if id == "ripe" {
			workers = 4
		} else if id == "caida" || id == iplookup.CloudflareID {
			workers = 2
		}
		queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "ip-lookup-" + id, Capacity: 32, MinInterval: 250 * time.Millisecond, Workers: workers})
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

func newIPLookupService(store *database.Store, settings *admin.SettingsService, queues *providerQueues, dispatcher *providerops.Dispatcher, key []byte) (*iplookup.Service, error) {
	providers := []iplookup.Provider{}
	for _, descriptor := range iplookup.Registry {
		providers = append(providers, iplookup.NewHTTPProvider(descriptor.ID, queues.iplookup[descriptor.ID], settings))
	}
	if err := dispatcher.Register(iplookup.OperationKind, iplookup.NewWorker(store, providers)); err != nil {
		return nil, err
	}
	service := iplookup.NewService(store, key)
	service.Live = iplookup.NewLiveService(settings, queues.iplookup)
	return service, nil
}
