package requestauth

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestReplayGlobalCapacityPreservesLiveEvidence(t *testing.T) {
	t.Parallel()
	cache := newReplayCache(10)
	cache.maxTotalNonces = 2
	now := time.Unix(100, 0)
	for _, session := range []string{"first", "second"} {
		if err := cache.add(session, "accepted", now, now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if err := cache.add("third", "new", now, now.Add(time.Minute)); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("new session at capacity: %v", err)
	}
	if err := cache.add("first", "new", now, now.Add(time.Minute)); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("existing session at capacity: %v", err)
	}
	if len(cache.sessions) != 2 || cache.totalNonces != 2 {
		t.Fatalf("rejected admission retained state: sessions=%d nonces=%d", len(cache.sessions), cache.totalNonces)
	}
	for _, session := range []string{"first", "second"} {
		if err := cache.add(session, "accepted", now, now.Add(time.Minute)); !errors.Is(err, ErrReplay) {
			t.Fatalf("live evidence was lost for %s: %v", session, err)
		}
	}
	if err := cache.add("third", "new", now.Add(time.Minute), now.Add(2*time.Minute)); err != nil {
		t.Fatalf("expired global capacity was not reclaimed: %v", err)
	}
	if len(cache.sessions) != 1 || cache.totalNonces != 1 {
		t.Fatalf("expired state retained: sessions=%d nonces=%d", len(cache.sessions), cache.totalNonces)
	}
}

func TestReplayExpiryAccountingBeforePeriodicCleanup(t *testing.T) {
	t.Parallel()
	for _, replaceNonce := range []bool{false, true} {
		t.Run(strconv.FormatBool(replaceNonce), func(t *testing.T) {
			cache := newReplayCache(1)
			cache.maxTotalNonces = 1
			now := time.Unix(100, 0)
			if err := cache.add("session", "old", now, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			nonce := "new"
			if replaceNonce {
				nonce = "old"
			}
			if err := cache.add("session", nonce, now.Add(time.Second), now.Add(time.Minute)); err != nil {
				t.Fatalf("expired session entry blocked replacement: %v", err)
			}
			if cache.totalNonces != 1 || len(cache.sessions["session"].nonces) != 1 {
				t.Fatalf("replacement accounting: %d", cache.totalNonces)
			}
			cache.cleanup(now.Add(time.Minute))
			if cache.totalNonces != 0 || len(cache.sessions) != 0 {
				t.Fatalf("cleanup accounting: nonces=%d sessions=%d", cache.totalNonces, len(cache.sessions))
			}
		})
	}
}

func TestReplayGlobalCapacityIsAtomicAcrossSessions(t *testing.T) {
	t.Parallel()
	cache := newReplayCache(2)
	cache.maxTotalNonces = 16
	now := time.Unix(100, 0)
	results := make(chan error, 64)
	var workers sync.WaitGroup
	for index := range 64 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			results <- cache.add(strconv.Itoa(index), "nonce", now, now.Add(time.Minute))
		}()
	}
	workers.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else if !errors.Is(err, ErrRateLimited) {
			t.Fatal(err)
		}
	}
	if accepted != 16 || cache.totalNonces != 16 || len(cache.sessions) != 16 {
		t.Fatalf("capacity exceeded: accepted=%d nonces=%d sessions=%d", accepted, cache.totalNonces, len(cache.sessions))
	}
}
