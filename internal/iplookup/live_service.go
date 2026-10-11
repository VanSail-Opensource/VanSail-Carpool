package iplookup

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/platform/upstreamqueue"
)

type liveAdmission struct {
	active bool
	starts []time.Time
	at     time.Time
}
type nameEntry struct {
	name          string
	expires, used time.Time
}

// LiveRepository stores compact shared evidence independently of paid receipts.
type LiveRepository interface {
	CachedIPDetails(context.Context, string) (*LiveDetails, error)
	SaveIPDetails(context.Context, string, LiveDetails, time.Time) (string, error)
}

// LiveService owns persistent enrichment and bounded source/name admission.
type LiveService struct {
	settings Settings
	queues   map[string]*upstreamqueue.Queue
	client   *http.Client
	resolver interface {
		LookupAddr(context.Context, string) ([]string, error)
	}
	mu         sync.Mutex
	users      map[string]*liveAdmission
	names      map[uint32]nameEntry
	repository LiveRepository
	flights    map[string]*liveFlight
	now        func() time.Time
}

// NewLiveService reuses the existing upstream queue implementation for every source.
func NewLiveService(settings Settings, queues map[string]*upstreamqueue.Queue, repositories ...LiveRepository) *LiveService {
	s := &LiveService{settings: settings, queues: queues, client: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}, resolver: net.DefaultResolver, users: map[string]*liveAdmission{}, names: map[uint32]nameEntry{}, flights: map[string]*liveFlight{}, now: time.Now}
	if len(repositories) > 0 {
		s.repository = repositories[0]
	}
	return s
}

func (s *LiveService) admit(user string) (func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, v := range s.users {
		if !v.active && now.Sub(v.at) >= time.Minute {
			delete(s.users, key)
		}
	}
	a := s.users[user]
	if a == nil {
		if len(s.users) >= 4096 {
			return nil, &CodeError{"IP_DETAILS_RATE_LIMITED"}
		}
		a = &liveAdmission{}
		s.users[user] = a
	}
	if a.active {
		return nil, &CodeError{"IP_DETAILS_BUSY"}
	}
	starts := a.starts[:0]
	for _, t := range a.starts {
		if now.Sub(t) < time.Minute {
			starts = append(starts, t)
		}
	}
	a.starts = starts
	if len(starts) >= 6 {
		return nil, &CodeError{"IP_DETAILS_RATE_LIMITED"}
	}
	a.active, a.at, a.starts = true, now, append(starts, now)
	return func() { s.mu.Lock(); a.active = false; s.mu.Unlock() }, nil
}

// Details reuses recent live data without creating a financial operation.
func (s *LiveService) Details(ctx context.Context, user, rawIP string) (LiveDetails, error) {
	return s.DetailsWithOptions(ctx, user, rawIP, LiveOptions{})
}

// DetailsWithOptions streams shared work and optionally bypasses completed cache entries.
func (s *LiveService) DetailsWithOptions(ctx context.Context, user, rawIP string, options LiveOptions) (LiveDetails, error) {
	if options.Refresh {
		return LiveDetails{}, &CodeError{"IP_LOOKUP_REFRESH_REQUIRED"}
	}
	ip, err := CanonicalIP(rawIP)
	if err != nil {
		return LiveDetails{}, err
	}
	raw, err := s.settings.Optional(ctx, SettingKey)
	if err != nil {
		return LiveDetails{}, &CodeError{"IP_DETAILS_UNAVAILABLE"}
	}
	c, err := DecodeConfig(raw)
	if err != nil {
		return LiveDetails{}, err
	}
	if !c.Enabled {
		return LiveDetails{}, &CodeError{"IP_LOOKUP_DISABLED"}
	}
	release, err := s.admit(user)
	if err != nil {
		return LiveDetails{}, err
	}
	defer release()
	if s.repository != nil {
		cached, err := s.repository.CachedIPDetails(ctx, ip)
		if err != nil {
			return LiveDetails{}, err
		}
		if cached != nil {
			return emitCachedDetails(*cached, options.Emit)
		}
	}
	return s.liveShared(ctx, ip, ip, c, options)
}
