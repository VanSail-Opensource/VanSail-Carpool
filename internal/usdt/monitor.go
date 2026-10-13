package usdt

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
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
		addresses, err := s.repository.USDTWatchAddresses(ctx, spec.Network)
		if err != nil {
			s.state(spec.Network, "error", "", ErrorCode(err))
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
			s.state(spec.Network, "disabled", "", "")
			continue
		}
		cursor, err := s.repository.USDTCursor(ctx, spec.Network)
		if err != nil {
			s.state(spec.Network, "error", "", ErrorCode(err))
			continue
		}
		completed := false
		for _, reader := range readers {
			batch, err := reader.scan(ctx, cursor, addresses)
			if err != nil {
				code := ErrorCode(err)
				state := "error"
				var pause *Pause
				if errors.As(err, &pause) {
					state = "paused"
				}
				s.state(spec.Network, state, cursor.Value, code)
				continue
			}
			if err := s.repository.SaveUSDTBatch(ctx, spec.Network, batch); err != nil {
				s.state(spec.Network, "error", cursor.Value, ErrorCode(err))
				break
			}
			state := "catching_up"
			if batch.CaughtUp {
				state = "ready"
			}
			s.state(spec.Network, state, batch.Cursor.Value, "")
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
					status := s.states[spec.Network]
					status.Connection = "websocket"
					s.states[spec.Network] = status
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

func (s *scanner) refreshSubscription(ctx context.Context, addresses []string) {
	if time.Now().Before(s.subRetry) {
		return
	}
	if len(addresses) > 128 {
		addresses = addresses[:128]
	}
	key := strings.Join(addresses, ",")
	s.client.mu.Lock()
	socket := s.client.ws
	s.client.mu.Unlock()
	if socket != s.subSocket {
		s.subscription = ""
	}
	if socket != nil {
		select {
		case <-socket.done:
			s.subscription = ""
		default:
		}
	}
	if key == s.subscription {
		return
	}
	// Replacing the connection drops old subscriptions instead of accumulating them.
	if socket != nil && s.subscription != "" {
		s.client.close()
	}
	s.subscription = ""
	if err := s.client.subscribe(ctx, addresses); err == nil {
		s.subscription = key
		s.client.mu.Lock()
		s.subSocket = s.client.ws
		s.client.mu.Unlock()
	} else {
		s.subRetry = time.Now().Add(30 * time.Second)
	}
}
