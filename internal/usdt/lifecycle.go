package usdt

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	runtimeconfig "github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/config"
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
	lastMemory := time.Time{}
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
		if time.Since(lastMemory) >= time.Minute {
			runtimeconfig.LogMemory(slog.Default(), "USDT scanner memory")
			lastMemory = time.Now()
		}
		c, err := s.config(ctx)
		if err != nil {
			s.report(err)
		} else if key := fingerprint(c); key != current {
			stop()
			if err := s.executor.configure(ctx, c.Concurrency); err != nil {
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
					for _, spec := range Registry {
						if spec.Network != n.Network {
							continue
						}
						networkConfig := n
						workers.Add(1)
						go func() {
							defer workers.Done()
							s.monitor(networkCtx, spec, networkConfig, c.Enabled && currencyEnabled(networkConfig, spec))
						}()
					}
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

// Status copies immutable network snapshots; reads never contact a chain.
func (s *Service) Status(ctx context.Context) (Status, error) {
	if _, err := s.config(ctx); err != nil {
		return Status{}, err
	}
	result := Status{Networks: []NetworkStatus{}}
	s.mu.RLock()
	for _, spec := range Registry {
		n, ok := s.states[spec.Rail]
		if !ok {
			n = NetworkStatus{Network: spec.Network, Token: spec.Token, Rail: spec.Rail, State: "disabled"}
		}
		result.Networks = append(result.Networks, n)
	}
	s.mu.RUnlock()
	return result, nil
}

func (s *Service) state(network, value, cursor, code string) {
	spec, ok := Network(network)
	if !ok {
		return
	}
	connection := "polling"
	if value == "disabled" {
		connection = "none"
	}
	now := time.Now().UTC()
	s.mu.Lock()
	s.states[spec.Rail] = NetworkStatus{Network: spec.Network, Token: spec.Token, Rail: spec.Rail, State: value, Cursor: cursor, ErrorCode: code, UpdatedAt: &now, Connection: connection}
	s.mu.Unlock()
}
