package connectivity

import (
	"context"
	"time"
)

const (
	configPollInterval = time.Second
	cleanupInterval    = 5 * time.Minute
	storageTimeout     = 5 * time.Second
	sourceTimeout      = time.Minute
	sourceCacheTTL     = 30 * time.Second
)

// Run recovers interrupted attempts, maintains retention, and owns its worker.
// ProviderQueues must start the supplied network queue before calling Run.
func (s *Service) Run(ctx context.Context) error {
	if ctx == nil || s.repository == nil || s.source == nil || s.probe == nil || s.queue == nil || s.settings == nil {
		return &CodeError{Code: "CONNECTIVITY_DEPENDENCIES_UNAVAILABLE"}
	}
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return &CodeError{Code: "CONNECTIVITY_ALREADY_STARTED"}
	}
	s.started = true
	s.mu.Unlock()
	startupCtx, cancelStartup := context.WithTimeout(ctx, storageTimeout)
	err := s.repository.RecoverConnectivityAttempts(startupCtx, s.now().UTC())
	if err == nil {
		err = s.repository.PruneConnectivityAttempts(startupCtx, s.now().UTC())
	}
	cancelStartup()
	if err != nil {
		return &CodeError{Code: "CONNECTIVITY_STORAGE_UNAVAILABLE", Err: err}
	}
	workerCtx, cancelWorker := context.WithCancel(ctx)
	s.mu.Lock()
	s.lifecycle, s.ready = workerCtx, true
	s.mu.Unlock()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		s.worker(workerCtx)
	}()
	defer func() {
		s.mu.Lock()
		s.ready = false
		s.cancelWorkLocked()
		s.mu.Unlock()
		cancelWorker()
		<-workerDone
	}()
	poll := time.NewTicker(configPollInterval)
	cleanup := time.NewTicker(cleanupInterval)
	defer poll.Stop()
	defer cleanup.Stop()
	s.observeConfiguration(workerCtx)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-poll.C:
			s.observeConfiguration(workerCtx)
		case <-s.configWake:
			s.observeConfiguration(workerCtx)
		case <-cleanup.C:
			s.cleanup(workerCtx)
		}
	}
}

func (s *Service) observeConfiguration(ctx context.Context) {
	s.mu.Lock()
	readCtx, cancel := context.WithTimeout(ctx, storageTimeout)
	cfg, err := s.readConfig(readCtx)
	cancel()
	if err != nil {
		s.cancelWorkLocked()
		s.loadError = ErrorCode(err)
		s.mu.Unlock()
		return
	}
	s.applyConfigLocked(cfg)
	due := cfg.ScheduledEnabled && cfg.ProxyUserID > 0 && s.pending == nil && s.active == nil &&
		(s.completed.IsZero() || !s.now().Before(s.completed.Add(time.Duration(cfg.IntervalSeconds)*time.Second)))
	s.mu.Unlock()
	if due {
		if _, err := s.Start(ctx, "scheduled"); err != nil {
			s.mu.Lock()
			s.loadError = ErrorCode(err)
			s.mu.Unlock()
		}
	}
}

func (s *Service) applyConfigLocked(cfg Config) {
	hash := ConfigHash(cfg)
	if s.hash == hash {
		return
	}
	s.cancelWorkLocked()
	s.generation++
	s.config, s.hash = cfg, hash
	s.completed = time.Time{}
	s.loadedAt = time.Time{}
	s.attemptedAt = time.Time{}
	s.loadError = ""
}

func (s *Service) cancelWorkLocked() {
	if s.active != nil {
		s.active.cancel()
	}
	if s.pending != nil {
		s.pending.cancel()
		s.pending = nil
	}
}

func (s *Service) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.runWake:
			for {
				s.mu.Lock()
				item := s.pending
				if item == nil || ctx.Err() != nil {
					s.mu.Unlock()
					break
				}
				s.pending, s.active = nil, item
				s.mu.Unlock()
				s.process(item)
				item.cancel()
			}
		}
	}
}

func (s *Service) cleanup(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, storageTimeout)
	err := s.repository.PruneConnectivityAttempts(cleanupCtx, s.now().UTC())
	cancel()
	s.mu.Lock()
	s.storageError = ""
	if err != nil {
		s.storageError = "CONNECTIVITY_STORAGE_UNAVAILABLE"
	}
	s.mu.Unlock()
}
