package httpapi

import (
	"net/http"
	"sync"
	"time"
)

const (
	authLimitCapacity = 6
	authLimitRefill   = 10 * time.Second
	authLimitMaxKeys  = 4096
)

type authLimitState struct {
	tokens float64
	at     time.Time
}

type authLimiter struct {
	mu     sync.Mutex
	states map[string]authLimitState
	now    func() time.Time
}

func newAuthLimiter() *authLimiter {
	return &authLimiter{states: make(map[string]authLimitState), now: time.Now}
}

func (limiter *authLimiter) allow(key string) bool {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	now := limiter.now()
	state, exists := limiter.states[key]
	if !exists {
		if len(limiter.states) >= authLimitMaxKeys {
			limiter.removeOldest()
		}
		state = authLimitState{tokens: authLimitCapacity, at: now}
	}
	elapsed := now.Sub(state.at)
	state.tokens = min(authLimitCapacity, state.tokens+float64(elapsed)/float64(authLimitRefill))
	state.at = now
	allowed := state.tokens >= 1
	if allowed {
		state.tokens--
	}
	limiter.states[key] = state
	return allowed
}

func (limiter *authLimiter) removeOldest() {
	var oldestKey string
	var oldest time.Time
	for key, state := range limiter.states {
		if oldestKey == "" || state.at.Before(oldest) {
			oldestKey, oldest = key, state.at
		}
	}
	delete(limiter.states, oldestKey)
}

func (s *Server) limitAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authLimiter.allow(requestClientIP(r)) {
			w.Header().Set("Retry-After", "10")
			s.writeError(w, r, http.StatusTooManyRequests, "AUTH_RATE_LIMITED", "Too many authentication attempts. Please retry shortly.")
			return
		}
		next.ServeHTTP(w, r)
	})
}
