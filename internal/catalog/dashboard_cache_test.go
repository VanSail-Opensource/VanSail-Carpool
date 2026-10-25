package catalog

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestDashboardCacheOwnerEntryAndByteBounds(t *testing.T) {
	s := NewService(nil, nil, time.Minute)
	for i := range dashboardCacheEntries + 10 {
		s.cacheDashboard(fmt.Sprint(i), RemoteDashboard{SubscriptionURL: fmt.Sprint(i)}, time.Unix(int64(i), 0))
	}
	if len(s.cache) != dashboardCacheEntries || s.cache["0"].value.SubscriptionURL != "" || s.cache["265"].value.SubscriptionURL != "265" {
		t.Fatal("owner/entry bound failed", len(s.cache))
	}
	before := s.cacheBytes
	s.InvalidateDashboard("265")
	if s.cacheBytes >= before {
		t.Fatal("invalidation retained budget")
	}
	s.cacheDashboard("large", RemoteDashboard{SubscriptionURL: strings.Repeat("x", dashboardCacheBytes/2)}, time.Now())
	s.cacheDashboard("other", RemoteDashboard{SubscriptionURL: strings.Repeat("x", dashboardCacheBytes/2)}, time.Now())
	if s.cacheBytes > dashboardCacheBytes {
		t.Fatal("byte budget exceeded", s.cacheBytes)
	}
	s.cacheDashboard("other", RemoteDashboard{SubscriptionURL: strings.Repeat("x", dashboardCacheBytes+1)}, time.Now())
	if _, found := s.cache["other"]; found {
		t.Fatal("oversized response retained an older snapshot")
	}
}
