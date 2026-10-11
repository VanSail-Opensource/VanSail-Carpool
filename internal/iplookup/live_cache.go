package iplookup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

type liveCacheEntry struct {
	data          []byte
	expires, used time.Time
}

type liveFlight struct {
	changed  chan struct{}
	events   []LiveEvent
	data     []byte
	err      error
	finished bool
	waiters  int
	cancel   context.CancelFunc
}

func (s *LiveService) liveCacheKey(ctx context.Context, ip, config string) (string, error) {
	h := sha256.New()
	for _, value := range []string{ip, config} {
		_, _ = h.Write([]byte(value + "\x00"))
	}
	for _, id := range []string{"ipapi", "abuseipdb", CloudflareID} {
		value, err := s.settings.Optional(ctx, CredentialKey(id))
		if err != nil {
			return "", &CodeError{"IP_DETAILS_UNAVAILABLE"}
		}
		_, _ = h.Write([]byte(value + "\x00"))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *LiveService) liveShared(ctx context.Context, key, ip string, c Config, options LiveOptions) (LiveDetails, error) {
	s.mu.Lock()
	if entry, ok := s.cache[key]; ok && !options.Refresh && s.now().Before(entry.expires) {
		entry.used = s.now()
		s.cache[key] = entry
		s.mu.Unlock()
		if options.Emit != nil {
			if err := options.Emit(liveComplete(ip, entry.data)); err != nil {
				return LiveDetails{}, err
			}
		}
		var result LiveDetails
		err := json.Unmarshal(entry.data, &result)
		return result, err
	}
	f := s.flights[key]
	if f == nil {
		if len(s.flights) >= 64 {
			s.mu.Unlock()
			return LiveDetails{}, &CodeError{"IP_DETAILS_BUSY"}
		}
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		f = &liveFlight{changed: make(chan struct{}), cancel: cancel}
		s.flights[key] = f
		go s.runLiveFlight(workCtx, key, ip, c, f)
	}
	f.waiters++
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		f.waiters--
		if f.waiters == 0 && !f.finished {
			f.cancel()
			if s.flights[key] == f {
				delete(s.flights, key)
			}
		}
	}()
	index := 0
	for {
		s.mu.Lock()
		events := append([]LiveEvent{}, f.events[index:]...)
		index = len(f.events)
		finished, changed, data, err := f.finished, f.changed, f.data, f.err
		s.mu.Unlock()
		for _, event := range events {
			if options.Emit != nil {
				if err := options.Emit(event); err != nil {
					return LiveDetails{}, err
				}
			}
		}
		if finished {
			var result LiveDetails
			if err == nil {
				err = json.Unmarshal(data, &result)
			}
			return result, err
		}
		select {
		case <-ctx.Done():
			return LiveDetails{}, ctx.Err()
		case <-changed:
		}
	}
}

func (s *LiveService) runLiveFlight(ctx context.Context, key, ip string, c Config, f *liveFlight) {
	defer f.cancel()
	emit := func(event LiveEvent) {
		s.mu.Lock()
		defer s.mu.Unlock()
		f.events = append(f.events, event)
		close(f.changed)
		f.changed = make(chan struct{})
	}
	workCtx := withLiveRequests(ctx)
	result := s.livePipeline(workCtx, ip, c, emit)
	requests := workCtx.Value(liveRequestsKey{}).(*liveRequests)
	requests.mu.Lock()
	failed := requests.failed
	requests.mu.Unlock()
	data, err := json.Marshal(result)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f.data, f.err, f.finished = data, err, true
	if s.flights[key] == f {
		delete(s.flights, key)
		if err == nil && !failed && liveCacheable(result) {
			s.storeLiveCache(key, data)
		}
	}
	if err == nil {
		f.events = append(f.events, liveComplete(ip, data))
	} else {
		f.events = append(f.events, LiveEvent{Type: "error", IP: ip, ErrorCode: "IP_DETAILS_UNAVAILABLE"})
	}
	close(f.changed)
}
