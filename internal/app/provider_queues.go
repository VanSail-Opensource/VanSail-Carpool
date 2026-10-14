package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

const (
	remnawaveQueueCapacity = 64
	embyQueueCapacity      = 32
	telegramQueueCapacity  = 64
	paymentQueueCapacity   = 32
	remnawavePace          = 50 * time.Millisecond
	embyPace               = 100 * time.Millisecond
	telegramPace           = 35 * time.Millisecond
	paymentPace            = 100 * time.Millisecond
)

type providerQueues struct {
	xnodeValidation *upstreamqueue.Queue
	iplookup        map[string]*upstreamqueue.Queue
	remnawave       *upstreamqueue.Queue
	emby            *upstreamqueue.Queue
	telegram        *upstreamqueue.Queue
	payment         *upstreamqueue.Queue
	turnstile       *upstreamqueue.Queue
	connectivity    *upstreamqueue.Queue
}

func newProviderQueues() (*providerQueues, error) {
	remnawaveQueue, err := upstreamqueue.New(upstreamqueue.Config{
		Name: "remnawave", Capacity: remnawaveQueueCapacity, MinInterval: remnawavePace,
	})
	if err != nil {
		return nil, fmt.Errorf("create Remnawave queue: %w", err)
	}
	embyQueue, err := upstreamqueue.New(upstreamqueue.Config{
		Name: "emby", Capacity: embyQueueCapacity, MinInterval: embyPace,
	})
	if err != nil {
		return nil, fmt.Errorf("create Emby queue: %w", err)
	}
	telegramQueue, err := upstreamqueue.New(upstreamqueue.Config{Name: "telegram", Capacity: telegramQueueCapacity, MinInterval: telegramPace})
	if err != nil {
		return nil, fmt.Errorf("create Telegram queue: %w", err)
	}
	paymentQueue, err := upstreamqueue.New(upstreamqueue.Config{Name: "payment", Capacity: paymentQueueCapacity, MinInterval: paymentPace})
	if err != nil {
		return nil, fmt.Errorf("create payment queue: %w", err)
	}
	turnstileQueue, err := upstreamqueue.New(upstreamqueue.Config{Name: "turnstile", Capacity: 32, MinInterval: 100 * time.Millisecond})
	if err != nil {
		return nil, fmt.Errorf("create Turnstile queue: %w", err)
	}
	connectivityQueue, err := upstreamqueue.New(upstreamqueue.Config{Name: "connectivity", Capacity: 8})
	if err != nil {
		return nil, fmt.Errorf("create connectivity queue: %w", err)
	}
	ipQueues, err := newIPLookupQueues()
	if err != nil {
		return nil, err
	}
	xnodeValidation, err := upstreamqueue.New(upstreamqueue.Config{Name: "xnode-validation", Capacity: 32})
	if err != nil {
		return nil, err
	}
	return &providerQueues{xnodeValidation: xnodeValidation, iplookup: ipQueues, remnawave: remnawaveQueue, emby: embyQueue, telegram: telegramQueue, payment: paymentQueue, turnstile: turnstileQueue, connectivity: connectivityQueue}, nil
}

func (q *providerQueues) start(ctx context.Context) error {
	if q == nil || q.remnawave == nil || q.emby == nil || q.telegram == nil || q.payment == nil || q.turnstile == nil || q.connectivity == nil {
		return errors.New("provider queues are incomplete")
	}
	if err := q.remnawave.Start(ctx); err != nil {
		return fmt.Errorf("start Remnawave queue: %w", err)
	}
	if err := q.emby.Start(ctx); err != nil {
		_ = q.remnawave.Shutdown(context.Background())
		return fmt.Errorf("start Emby queue: %w", err)
	}
	if err := q.telegram.Start(ctx); err != nil {
		_ = q.emby.Shutdown(context.Background())
		_ = q.remnawave.Shutdown(context.Background())
		return fmt.Errorf("start Telegram queue: %w", err)
	}
	if err := q.payment.Start(ctx); err != nil {
		_ = q.telegram.Shutdown(context.Background())
		_ = q.emby.Shutdown(context.Background())
		_ = q.remnawave.Shutdown(context.Background())
		return fmt.Errorf("start payment queue: %w", err)
	}
	if err := q.turnstile.Start(ctx); err != nil {
		_ = q.payment.Shutdown(context.Background())
		_ = q.telegram.Shutdown(context.Background())
		_ = q.emby.Shutdown(context.Background())
		_ = q.remnawave.Shutdown(context.Background())
		return fmt.Errorf("start Turnstile queue: %w", err)
	}
	if err := q.connectivity.Start(ctx); err != nil {
		_ = q.shutdown(context.Background())
		return fmt.Errorf("start connectivity queue: %w", err)
	}
	if err := q.startIPLookup(ctx); err != nil {
		_ = q.shutdown(context.Background())
		return err
	}
	if q.xnodeValidation != nil {
		if err := q.xnodeValidation.Start(ctx); err != nil {
			_ = q.shutdown(context.Background())
			return err
		}
	}
	return nil
}

func (q *providerQueues) shutdown(ctx context.Context) error {
	if q == nil {
		return nil
	}
	errorsByProvider := make(chan error, 8)
	if q.xnodeValidation != nil {
		go func() { errorsByProvider <- q.xnodeValidation.Shutdown(ctx) }()
	} else {
		errorsByProvider <- nil
	}
	go func() { errorsByProvider <- q.shutdownIPLookup(ctx) }()
	go func() { errorsByProvider <- q.remnawave.Shutdown(ctx) }()
	go func() { errorsByProvider <- q.emby.Shutdown(ctx) }()
	go func() { errorsByProvider <- q.telegram.Shutdown(ctx) }()
	go func() { errorsByProvider <- q.payment.Shutdown(ctx) }()
	go func() { errorsByProvider <- q.turnstile.Shutdown(ctx) }()
	go func() { errorsByProvider <- q.connectivity.Shutdown(ctx) }()
	return errors.Join(<-errorsByProvider, <-errorsByProvider, <-errorsByProvider, <-errorsByProvider, <-errorsByProvider, <-errorsByProvider, <-errorsByProvider, <-errorsByProvider)
}
