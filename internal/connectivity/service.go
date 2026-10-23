package connectivity

import (
	"context"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	"github.com/google/uuid"
)

type work struct {
	run        Run
	config     Config
	hash       string
	generation uint64
	ctx        context.Context
	cancel     context.CancelFunc
}

// Service owns a single batch worker and durable, bounded diagnostic results.
// The supplied provider queue owns the network-probe admission lifecycle.
type Service struct {
	settings     Settings
	source       Source
	probe        Probe
	repository   Repository
	queue        *upstreamqueue.Queue
	now          func() time.Time
	retryWait    func(context.Context, time.Duration) error
	mu           sync.RWMutex
	sourceGate   chan struct{}
	started      bool
	ready        bool
	lifecycle    context.Context
	config       Config
	hash         string
	generation   uint64
	active       *work
	pending      *work
	last         *Run
	lastHash     string
	completed    time.Time
	user         *User
	targets      []Target
	loadedAt     time.Time
	attemptedAt  time.Time
	attemptHash  string
	loadHash     string
	loadError    string
	storageError string
	runWake      chan struct{}
	configWake   chan struct{}
}

// NewService composes runtime dependencies without starting any goroutines.
func NewService(settings Settings, source Source, probe Probe, repository Repository, queue *upstreamqueue.Queue) *Service {
	return &Service{settings: settings, source: source, probe: probe, repository: repository, queue: queue,
		now: time.Now, retryWait: waitForRetry, sourceGate: make(chan struct{}, 1), runWake: make(chan struct{}, 1), configWake: make(chan struct{}, 1)}
}

// Start accepts a process-owned batch, returning the active run for duplicates.
// Execution remains attached to Run's lifecycle after the HTTP caller returns.
func (s *Service) Start(ctx context.Context, trigger string) (Run, error) {
	if trigger != "manual" && trigger != "scheduled" {
		return Run{}, &CodeError{Code: "CONNECTIVITY_INVALID_TRIGGER"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.readConfig(ctx)
	if err != nil {
		return Run{}, err
	}
	if cfg.ProxyUserID == 0 {
		return Run{}, &CodeError{Code: "CONNECTIVITY_NOT_CONFIGURED"}
	}
	if trigger == "scheduled" && !cfg.ScheduledEnabled {
		return Run{}, &CodeError{Code: "CONNECTIVITY_SCHEDULING_DISABLED"}
	}
	if !s.ready || s.lifecycle.Err() != nil {
		return Run{}, &CodeError{Code: "CONNECTIVITY_NOT_READY"}
	}
	s.applyConfigLocked(cfg)
	if s.pending != nil {
		return cloneRun(s.pending.run), nil
	}
	if s.active != nil && s.active.generation == s.generation {
		return cloneRun(s.active.run), nil
	}
	runCtx, cancel := context.WithCancel(s.lifecycle)
	s.pending = &work{run: Run{ID: uuid.NewString(), Status: "running", Trigger: trigger, StartedAt: s.now().UTC()},
		config: cfg, hash: s.hash, generation: s.generation, ctx: runCtx, cancel: cancel}
	signal(s.runWake)
	return cloneRun(s.pending.run), nil
}

// Invalidate cancels obsolete work immediately after an atomic settings change.
func (s *Service) Invalidate() {
	s.mu.Lock()
	s.cancelWorkLocked()
	s.generation++
	s.hash = ""
	s.completed = time.Time{}
	s.loadedAt = time.Time{}
	s.attemptedAt = time.Time{}
	s.mu.Unlock()
	signal(s.configWake)
}

// Resolve validates an exact existing upstream username without returning secrets.
func (s *Service) Resolve(ctx context.Context, username string) (User, error) {
	if ctx == nil || s.source == nil {
		return User{}, &CodeError{Code: "CONNECTIVITY_ACCOUNT_UNAVAILABLE"}
	}
	return s.source.Resolve(ctx, username)
}

// ValidateConfig checks the typed settings and selected upstream account.
func (s *Service) ValidateConfig(ctx context.Context, cfg Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}
	if cfg.ProxyUserID == 0 {
		return nil
	}
	if s.source == nil || ctx == nil {
		return &CodeError{Code: "CONNECTIVITY_ACCOUNT_UNAVAILABLE"}
	}
	return s.source.Validate(ctx, cfg.ProxyUserID)
}

func (s *Service) readConfig(ctx context.Context) (Config, error) {
	if ctx == nil || s.settings == nil {
		return Config{}, &CodeError{Code: "CONNECTIVITY_SETTINGS_UNAVAILABLE"}
	}
	readCtx, cancel := context.WithTimeout(ctx, storageTimeout)
	defer cancel()
	value, err := s.settings.Optional(readCtx, SettingKey)
	if err != nil {
		return Config{}, &CodeError{Code: "CONNECTIVITY_SETTINGS_UNAVAILABLE", Err: err}
	}
	return DecodeConfig(value)
}

func cloneRun(run Run) Run {
	if run.FinishedAt != nil {
		finished := *run.FinishedAt
		run.FinishedAt = &finished
	}
	return run
}

func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
