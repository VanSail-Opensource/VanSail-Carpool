package iplookup

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestLiveCompletionRetainsDegradedTasksForFreshAndCachedResults(t *testing.T) {
	s, _, _ := integratedLiveService(t, false)
	s.repository = &snapshotMemory{}
	s.resolver = liveResolver{absent: true}
	s.settings.(liveConfigSettings)[CredentialKey("ipapi")] = ""
	original := s.client.Transport
	s.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status, body := 200, ""
		switch {
		case r.URL.Host == "internetdb.shodan.io":
			status, body = 503, `{}`
		case strings.HasSuffix(r.URL.Path, "/summary/BOT_CLASS"):
			body = `{"success":true,"result":{"summary_0":{"bot":"invalid"}}}`
		default:
			return original.RoundTrip(r)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})
	for _, name := range []string{"fresh", "cached"} {
		t.Run(name, func(t *testing.T) {
			var complete *Progress
			_, err := s.DetailsWithOptions(context.Background(), "member", "1.1.1.1", LiveOptions{Emit: func(event LiveEvent) error {
				if event.Type == "complete" {
					complete = event.Progress
				}
				return nil
			}})
			if err != nil || complete == nil || complete.Phase != "complete" || complete.Completed != len(livePhases) || len(complete.Tasks) != len(livePhases) {
				t.Fatalf("missing completed progress: %+v, %v", complete, err)
			}
			tasks := map[string]ProgressTask{}
			for _, task := range complete.Tasks {
				tasks[task.Key] = task
			}
			for phase, want := range map[string]string{"shodan": "error", "asns": "partial", "scores": "skipped", "ptr": "success", "transfers": "success"} {
				if tasks[phase].Status != want {
					t.Errorf("%s = %+v, want %s", phase, tasks[phase], want)
				}
			}
			if tasks["shodan"].ErrorCode != "IP_DETAILS_UNAVAILABLE" {
				t.Fatal("completion discarded the source failure reason")
			}
		})
	}
}

func TestASNTaskTracksPartialSourcesAndTrafficWithoutInventingFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		asns   []ASNDetails
		status string
	}{
		{"empty", nil, "success"},
		{"partial source", []ASNDetails{{Sources: map[string]LiveMeta{"routing": {Status: "partial"}}}}, "partial"},
		{"all sources failed", []ASNDetails{{Sources: map[string]LiveMeta{"routing": {Status: "error"}}}}, "error"},
		{"mixed origins", []ASNDetails{{Sources: map[string]LiveMeta{"routing": {Status: "error"}}}, {Sources: map[string]LiveMeta{"routing": {Status: "success"}}}}, "partial"},
		{"failed traffic", []ASNDetails{{Traffic: ASNTraffic{Devices: TrafficSummary{LiveMeta: LiveMeta{Status: "error"}}}}}, "partial"},
		{"optional traffic", []ASNDetails{{Traffic: ASNTraffic{Devices: TrafficSummary{LiveMeta: LiveMeta{Status: "unconfigured"}}}}}, "success"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := liveTaskStatus("asns", tc.asns); got.Status != tc.status {
				t.Fatalf("task = %+v, want %s", got, tc.status)
			}
		})
	}
}
