package iplookup

import (
	"context"
	"encoding/json"
	"time"
)

type liveFlight struct {
	changed    chan struct{}
	events     []LiveEvent
	data       []byte
	err        error
	finished   bool
	waiters    int
	cancel     context.CancelFunc
	snapshotID string
}

// RefreshRun is called only by an admitted, charged durable operation.
func (s *LiveService) RefreshRun(ctx context.Context, ip string, c Config, run string, attempt func(context.Context) error) (LiveDetails, error) {
	return s.liveShared(ctx, "refresh:"+run, ip, c, LiveOptions{runKey: run, onAttempt: attempt})
}

func (s *LiveService) liveShared(ctx context.Context, key, ip string, c Config, options LiveOptions) (LiveDetails, error) {
	s.mu.Lock()
	f := s.flights[key]
	if f == nil {
		if len(s.flights) >= 64 {
			s.mu.Unlock()
			return LiveDetails{}, &CodeError{"IP_DETAILS_BUSY"}
		}
		workCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		f = &liveFlight{changed: make(chan struct{}), cancel: cancel}
		s.flights[key] = f
		go s.runLiveFlight(workCtx, key, ip, c, f, options)
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
				result.SnapshotID = f.snapshotID
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

func (s *LiveService) runLiveFlight(ctx context.Context, key, ip string, c Config, f *liveFlight, options LiveOptions) {
	defer f.cancel()
	emit := func(event LiveEvent) {
		s.mu.Lock()
		defer s.mu.Unlock()
		f.events = append(f.events, event)
		close(f.changed)
		f.changed = make(chan struct{})
	}
	workCtx := withLiveRequests(ctx)
	requests := workCtx.Value(liveRequestsKey{}).(*liveRequests)
	requests.onAttempt = options.onAttempt
	result := s.livePipeline(workCtx, ip, c, emit)
	requests.mu.Lock()
	attempted, attemptErr := requests.attempted, requests.attemptErr
	requests.mu.Unlock()
	data, err := json.Marshal(result)
	if attemptErr != nil {
		err = attemptErr
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	var snapshotID string
	if err == nil && attempted && s.repository != nil {
		snapshotID, err = s.repository.SaveIPDetails(ctx, options.runKey, result, s.now().UTC())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	f.data, f.err, f.finished, f.snapshotID = data, err, true, snapshotID
	if s.flights[key] == f {
		delete(s.flights, key)
	}
	if err == nil {
		f.events = append(f.events, liveComplete(ip, data))
	} else {
		f.events = append(f.events, LiveEvent{Type: "error", IP: ip, ErrorCode: "IP_DETAILS_UNAVAILABLE"})
	}
	close(f.changed)
}
