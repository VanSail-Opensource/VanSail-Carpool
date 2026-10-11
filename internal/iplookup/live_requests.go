package iplookup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/url"
	"sync"
)

type liveRequestsKey struct{}
type liveRequestResult struct {
	done chan struct{}
	data []byte
	err  error
}
type liveRequests struct {
	mu          sync.Mutex
	results     map[[32]byte]*liveRequestResult
	failed      bool
	attempted   bool
	attemptOnce sync.Once
	attemptErr  error
	onAttempt   func(context.Context) error
}

func markLiveAttempt(ctx context.Context) error {
	memo, ok := ctx.Value(liveRequestsKey{}).(*liveRequests)
	if !ok {
		return nil
	}
	memo.attemptOnce.Do(func() {
		var err error
		if memo.onAttempt != nil {
			err = memo.onAttempt(ctx)
		}
		memo.mu.Lock()
		memo.attemptErr, memo.attempted = err, err == nil
		memo.mu.Unlock()
	})
	return memo.attemptErr
}

func withLiveRequests(ctx context.Context) context.Context {
	return context.WithValue(ctx, liveRequestsKey{}, &liveRequests{results: map[[32]byte]*liveRequestResult{}})
}

// liveHTTP coalesces identical reads only within this cancellable lookup.
func (s *LiveService) liveHTTP(ctx context.Context, source, path string, query url.Values, body any, credential string) ([]byte, error) {
	memo, ok := ctx.Value(liveRequestsKey{}).(*liveRequests)
	if !ok {
		return s.liveHTTPRequest(ctx, source, path, query, body, credential)
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, &CodeError{"IP_DETAILS_INVALID_RESPONSE"}
	}
	key := sha256.Sum256([]byte(source + "\x00" + path + "\x00" + query.Encode() + "\x00" + string(payload) + "\x00" + credential))
	memo.mu.Lock()
	entry := memo.results[key]
	if entry != nil {
		memo.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-entry.done:
			return entry.data, entry.err
		}
	}
	entry = &liveRequestResult{done: make(chan struct{})}
	memo.results[key] = entry
	memo.mu.Unlock()
	entry.data, entry.err = s.liveHTTPRequest(ctx, source, path, query, body, credential)
	if entry.err != nil && ErrorCode(entry.err) != "IP_DETAILS_NOT_FOUND" && ErrorCode(entry.err) != "IP_DETAILS_PLAN_RESTRICTED" {
		memo.mu.Lock()
		memo.failed = true
		memo.mu.Unlock()
	}
	close(entry.done)
	return entry.data, entry.err
}
