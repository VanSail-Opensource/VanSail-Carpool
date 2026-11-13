package usdt

import (
	"context"
	"strings"
)

// A rail retains only its selected endpoint's notification reader. Retiring a
// socket does not close the shared HTTP transport's otherwise reusable pools.
func closeInactiveSubscriptions(readers []*scanner, selected *scanner) {
	for _, reader := range readers {
		if reader != selected {
			reader.closeSubscription()
		}
	}
}

func (s *scanner) closeSubscription() {
	s.client.closeSocket()
	s.subscription = ""
	s.subSocket = nil
}

func (s *scanner) refreshSubscription(ctx context.Context, addresses []string) {
	if len(addresses) > 128 {
		addresses = addresses[:128]
	}
	key := strings.Join(addresses, ",")
	if key == "" {
		s.closeSubscription()
		return
	}
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
		s.closeSubscription()
	}
	s.subscription = ""
	if err := s.client.subscribe(ctx, addresses); err == nil {
		s.subscription = key
		s.client.mu.Lock()
		s.subSocket = s.client.ws
		s.client.mu.Unlock()
	} else {
		// A partial multi-account subscription must not accumulate on retry.
		s.closeSubscription()
	}
}
