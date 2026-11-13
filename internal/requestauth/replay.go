package requestauth

import (
	"sync"
	"time"
)

type sessionReplays struct {
	nonces map[string]time.Time
}

type replayCache struct {
	mu             sync.Mutex
	sessions       map[string]*sessionReplays
	maxNonces      int
	maxTotalNonces int
	totalNonces    int
	nextCleanup    time.Time
}

func newReplayCache(maxNonces int) *replayCache {
	return &replayCache{
		sessions: make(map[string]*sessionReplays), maxNonces: maxNonces,
		maxTotalNonces: 32768,
	}
}

func (cache *replayCache) add(sessionKey, nonce string, now, expiresAt time.Time) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if !now.Before(cache.nextCleanup) {
		cache.cleanup(now)
	}
	session := cache.sessions[sessionKey]
	if session != nil {
		if expiry, exists := session.nonces[nonce]; exists {
			if expiry.After(now) {
				return ErrReplay
			}
			delete(session.nonces, nonce)
			cache.totalNonces--
		}
		if len(session.nonces) >= cache.maxNonces {
			cache.expireSession(session, now)
		}
		if len(session.nonces) >= cache.maxNonces {
			return ErrRateLimited
		}
	}
	// Fail closed at the global budget. Never evict live evidence to admit a
	// different session: doing so would allow already accepted requests to replay.
	// Periodic cleanup bounds both memory and the work spent on rejected requests.
	if cache.totalNonces >= cache.maxTotalNonces {
		return ErrRateLimited
	}
	if session == nil {
		session = &sessionReplays{nonces: make(map[string]time.Time)}
		cache.sessions[sessionKey] = session
	}
	session.nonces[nonce] = expiresAt
	cache.totalNonces++
	return nil
}

func (cache *replayCache) expireSession(session *sessionReplays, now time.Time) {
	for nonce, expiry := range session.nonces {
		if !expiry.After(now) {
			delete(session.nonces, nonce)
			cache.totalNonces--
		}
	}
}

func (cache *replayCache) cleanup(now time.Time) {
	for key, session := range cache.sessions {
		cache.expireSession(session, now)
		if len(session.nonces) == 0 {
			delete(cache.sessions, key)
		}
	}
	cache.nextCleanup = now.Add(time.Minute)
}
