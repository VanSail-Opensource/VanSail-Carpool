package usdt

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Start attaches every scanner and queue to the application context.
func (s *Service) Start(ctx context.Context) { s.start.Do(func() { go s.run(ctx) }) }

// Wait joins all readers before SQLite is closed.
func (s *Service) Wait() { <-s.done }

func (s *Service) run(ctx context.Context) {
	defer close(s.done)
	defer func() { _ = s.executor.close(context.Background()) }()
	var cancel context.CancelFunc
	var workers sync.WaitGroup
	current := ""
	stop := func() {
		if cancel != nil {
			cancel()
			workers.Wait()
			cancel = nil
		}
	}
	defer stop()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		c, err := s.config(ctx)
		if err != nil {
			s.report(err)
		} else if key := fingerprint(c); key != current {
			stop()
			if err := s.executor.configure(ctx, c.Groups); err != nil {
				s.report(err)
			} else {
				current = key
				networkCtx, networkCancel := context.WithCancel(ctx)
				cancel = networkCancel
				s.mu.Lock()
				s.fingerprint = key
				s.states = map[string]NetworkStatus{}
				s.mu.Unlock()
				for _, n := range c.Networks {
					spec, _ := Network(n.Network)
					networkConfig := n
					workers.Add(1)
					go func() { defer workers.Done(); s.monitor(networkCtx, spec, networkConfig, c.Enabled) }()
				}
			}
		}
		if err := s.drain(ctx); err != nil && !errors.Is(err, context.Canceled) {
			s.report(err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
			current = ""
		}
	}
}

func (s *Service) drain(ctx context.Context) error {
	items, err := s.repository.USDTReady(ctx, 100)
	if err != nil {
		return err
	}
	var failures []error
	for _, t := range items {
		if _, _, err := s.repository.SettleUSDTTransfer(ctx, t.ID, t.OrderID, "", ""); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
func (s *Service) report(err error) {
	if s.OnError != nil {
		s.OnError(ErrorCode(err))
	}
}

// Status derives quota counters locally and copies immutable network snapshots.
func (s *Service) Status(ctx context.Context) (Status, error) {
	c, err := s.config(ctx)
	if err != nil {
		return Status{}, err
	}
	result := Status{Networks: []NetworkStatus{}, Quotas: []QuotaStatus{}}
	s.mu.RLock()
	for _, spec := range Registry {
		n, ok := s.states[spec.Network]
		if !ok {
			n = NetworkStatus{Network: spec.Network, State: "disabled"}
		}
		result.Networks = append(result.Networks, n)
	}
	s.mu.RUnlock()
	result.Quotas, err = s.repository.USDTQuotaStatus(ctx, c.Groups, time.Now().UTC())
	return result, err
}

func (s *Service) state(network, value, cursor, code string) {
	connection := "polling"
	if value == "disabled" {
		connection = "none"
	}
	now := time.Now().UTC()
	s.mu.Lock()
	s.states[network] = NetworkStatus{Network: network, State: value, Cursor: cursor, ErrorCode: code, UpdatedAt: &now, Connection: connection}
	s.mu.Unlock()
}
