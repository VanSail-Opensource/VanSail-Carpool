package usdt

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

func (s *Service) monitor(ctx context.Context, spec NetworkSpec, n NetworkConfig, enabled bool) {
	wake := make(chan struct{}, 1)
	readers := []*scanner{}
	for _, e := range n.Endpoints {
		client := newClient(ctx, s.executor, e, spec)
		client.wake = wake
		readers = append(readers, &scanner{client: client})
	}
	defer func() {
		for _, r := range readers {
			r.client.close()
		}
	}()
	delay := time.Duration(0)
	lastScan := time.Time{}
	for {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			case <-wake:
				timer.Stop()
				if remaining := 5*time.Second - time.Since(lastScan); remaining > 0 {
					coalesce := time.NewTimer(remaining)
					select {
					case <-ctx.Done():
						coalesce.Stop()
						return
					case <-coalesce.C:
					}
				}
			}
		}
		if ctx.Err() != nil {
			return
		}
		delay = 15 * time.Second
		lastScan = time.Now()
		addresses, err := s.repository.USDTWatchAddresses(ctx, spec.Rail)
		if err != nil {
			s.state(spec.Rail, "error", "", ErrorCode(err))
			continue
		}
		if enabled && n.Enabled {
			for _, a := range n.Addresses {
				if !containsAddress(addresses, a) {
					addresses = append(addresses, a)
				}
			}
		}
		sort.Strings(addresses)
		if len(addresses) == 0 || len(readers) == 0 {
			closeInactiveSubscriptions(readers, nil)
			s.state(spec.Rail, "disabled", "", "")
			continue
		}
		cursor, err := s.repository.USDTCursor(ctx, spec.Rail)
		if err != nil {
			s.state(spec.Rail, "error", "", ErrorCode(err))
			continue
		}
		completed := false
		for _, reader := range readers {
			batch, err := reader.scan(ctx, cursor, addresses)
			if err != nil {
				reader.closeSubscription()
				code := ErrorCode(err)
				s.state(spec.Rail, "error", cursor.Value, code)
				continue
			}
			closeInactiveSubscriptions(readers, reader)
			if err := s.repository.SaveUSDTBatch(ctx, spec.Rail, batch); err != nil {
				s.state(spec.Rail, "error", cursor.Value, ErrorCode(err))
				break
			}
			state := "catching_up"
			if batch.CaughtUp {
				state = "ready"
			}
			s.state(spec.Rail, state, batch.Cursor.Value, "")
			completed = true
			if err := s.drain(ctx); err != nil {
				s.report(err)
			}
			if !batch.CaughtUp {
				delay = time.Second
			}
			if reader.client.endpoint.UseWS {
				keys := addresses
				if spec.Family == "solana" {
					var c solCursor
					if json.Unmarshal([]byte(batch.Cursor.Value), &c) == nil {
						keys = []string{}
						for account, h := range c.Accounts {
							if containsAddress(addresses, h.Owner) {
								keys = append(keys, account)
							}
						}
						sort.Strings(keys)
					}
				}
				reader.refreshSubscription(ctx, keys)
				if reader.subscription != "" {
					s.mu.Lock()
					status := s.states[spec.Rail]
					status.Connection = "websocket"
					s.states[spec.Rail] = status
					s.mu.Unlock()
				}
			}
			break
		}
		if !completed {
			delay = 15 * time.Second
		}
	}
}
