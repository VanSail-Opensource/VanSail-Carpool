package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"time"
)

const (
	embyQueueCapacity     = 32
	telegramQueueCapacity = 64
	paymentQueueCapacity  = 32
	embyPace              = 100 * time.Millisecond
	telegramPace          = 35 * time.Millisecond
	paymentPace           = 100 * time.Millisecond
)

type providerQueues struct {
	xnodeValidation                                  *upstreamqueue.Queue
	iplookup                                         map[string]*upstreamqueue.Queue
	emby, telegram, payment, turnstile, connectivity *upstreamqueue.Queue
}

func newProviderQueues() (*providerQueues, error) {
	q := &providerQueues{}
	entries := []struct {
		target **upstreamqueue.Queue
		config upstreamqueue.Config
	}{
		{&q.emby, upstreamqueue.Config{Name: "emby", Capacity: embyQueueCapacity, MinInterval: embyPace, Workers: 4}},
		{&q.telegram, upstreamqueue.Config{Name: "telegram", Capacity: telegramQueueCapacity, MinInterval: telegramPace, Workers: 4}},
		{&q.payment, upstreamqueue.Config{Name: "payment", Capacity: paymentQueueCapacity, MinInterval: paymentPace, Workers: 4}},
		{&q.turnstile, upstreamqueue.Config{Name: "turnstile", Capacity: 32, MinInterval: 100 * time.Millisecond, Workers: 4}},
		{&q.connectivity, upstreamqueue.Config{Name: "connectivity", Capacity: 8, Workers: 4}},
		{&q.xnodeValidation, upstreamqueue.Config{Name: "xnode-validation", Capacity: 32, Workers: 4}},
	}
	for _, entry := range entries {
		queue, err := upstreamqueue.New(entry.config)
		if err != nil {
			return nil, fmt.Errorf("create %s queue: %w", entry.config.Name, err)
		}
		*entry.target = queue
	}
	var err error
	q.iplookup, err = newIPLookupQueues()
	return q, err
}
func (q *providerQueues) queues() []*upstreamqueue.Queue {
	return []*upstreamqueue.Queue{q.emby, q.telegram, q.payment, q.turnstile, q.connectivity, q.xnodeValidation}
}
func (q *providerQueues) start(ctx context.Context) error {
	if q == nil {
		return errors.New("provider queues are incomplete")
	}
	for _, queue := range q.queues() {
		if queue == nil {
			return errors.New("provider queues are incomplete")
		}
	}
	for _, queue := range q.queues() {
		if err := queue.Start(ctx); err != nil {
			_ = q.shutdown(context.Background())
			return fmt.Errorf("start %s queue: %w", queue.Name(), err)
		}
	}
	if err := q.startIPLookup(ctx); err != nil {
		_ = q.shutdown(context.Background())
		return err
	}
	return nil
}
func (q *providerQueues) shutdown(ctx context.Context) error {
	if q == nil {
		return nil
	}
	results := make(chan error, len(q.queues())+1)
	for _, queue := range q.queues() {
		go func() { results <- queue.Shutdown(ctx) }()
	}
	go func() { results <- q.shutdownIPLookup(ctx) }()
	var result error
	for range len(q.queues()) + 1 {
		result = errors.Join(result, <-results)
	}
	return result
}
