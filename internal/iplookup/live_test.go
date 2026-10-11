package iplookup

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLiveTopologyLongestPrefixAndOriginalPaths(t *testing.T) {
	for _, tc := range []struct{ ip, parent, prefix string }{
		{"1.1.1.1", "1.0.0.0/8", "1.1.1.0/24"},
		{"2606:4700:4700::1111", "2606:4700::/32", "2606:4700:4700::/48"},
	} {
		t.Run(tc.ip, func(t *testing.T) {
			rows := []routeObservation{{tc.parent, BGPPath{ASPath: []uint32{999, 888}}}, {tc.prefix, BGPPath{ASPath: []uint32{174, 13335, 13335}, Collector: "rrc01", Exchange: "LINX"}}, {tc.prefix, BGPPath{ASPath: []uint32{3257, 48266}}}}
			graph := buildTopology(tc.ip, rows, LiveMeta{Status: "success"})
			if graph.Prefix != tc.prefix || len(graph.Origins) != 2 || len(graph.Nodes) != 4 || len(graph.Edges) != 2 || len(graph.Paths[0].ASPath) != 3 {
				t.Fatalf("incorrect topology: %+v", graph)
			}
			if graph.Edges[0].Relationship != "observed" || graph.Paths[0].Exchange != "LINX" {
				t.Fatal("invented relation or lost collector context")
			}
		})
	}
	if asPath("174 {13335,48266}") != nil {
		t.Fatal("must not fabricate adjacency across AS sets")
	}
}

func TestLiveAdmissionReleasesAndBoundsStarts(t *testing.T) {
	now := time.Now()
	s := NewLiveService(nil, nil)
	s.now = func() time.Time { return now }
	release, err := s.admit("a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.admit("a"); ErrorCode(err) != "IP_DETAILS_BUSY" {
		t.Fatalf("busy: %v", err)
	}
	release()
	for i := 0; i < 5; i++ {
		release, err = s.admit("a")
		if err != nil {
			t.Fatal(err)
		}
		release()
	}
	if _, err := s.admit("a"); ErrorCode(err) != "IP_DETAILS_RATE_LIMITED" {
		t.Fatalf("limit: %v", err)
	}
	now = now.Add(time.Minute)
	release, err = s.admit("a")
	if err != nil {
		t.Fatal(err)
	}
	release()
	_, err = s.Details(context.Background(), "a", "127.0.0.1")
	if err == nil {
		t.Fatal("accepted private IP")
	}
}

func TestLiveNameCacheHasIndependentExpiryAndBounds(t *testing.T) {
	now := time.Now()
	s := NewLiveService(nil, nil)
	s.now = func() time.Time { return now }
	for i := uint32(1); i <= 4097; i++ {
		s.rememberName(i, "Network "+asnText(i))
		now = now.Add(time.Millisecond)
	}
	if len(s.names) != 4096 || s.cachedName(1) != "" || s.cachedName(4097) == "" {
		t.Fatal("name cache bounds/eviction")
	}
	now = now.Add(7 * 24 * time.Hour)
	if s.cachedName(4097) != "" {
		t.Fatal("expired name reused")
	}
}

func TestLiveScoreUnknownZeroAndFinite(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		known bool
	}{{"0 (Low)", true}, {"0.031 (Elevated)", true}, {"Premium field required", false}, {"NaN", false}, {"Infinity", false}, {"-0.2", false}} {
		score := score(liveObject([]byte(`{"abuser_score":"` + tc.raw + `"}`)))
		if (score.Ratio != nil) != tc.known {
			t.Fatalf("%s: %+v", tc.raw, score)
		}
		if _, err := json.Marshal(score); err != nil {
			t.Fatal(err)
		}
	}
	if err := errors.New("https://secret.example/?key=private"); strings.Contains(ErrorCode(err), "private") {
		t.Fatal("leaked upstream error")
	}
}
