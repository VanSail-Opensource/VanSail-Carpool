package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
)

type detailsSettings struct{}

func (detailsSettings) Optional(_ context.Context, key string) (string, error) {
	if key == iplookup.SettingKey {
		return `{"enabled":true,"lookupFeeTxb":"0","refreshFeeTxb":"0","providers":[{"id":"ipapi","enabled":true}]}`, nil
	}
	return "", nil
}

type detailsRecorder struct {
	*httptest.ResponseRecorder
	deadline time.Time
}

func (w *detailsRecorder) SetWriteDeadline(at time.Time) error { w.deadline = at; return nil }

func TestLiveDetailsNegotiatesStreamAndJSONWithRouteDeadline(t *testing.T) {
	for _, accept := range []string{"application/json", "text/event-stream"} {
		t.Run(accept, func(t *testing.T) {
			s := &Server{deps: Dependencies{IPLookup: &iplookup.Service{Live: iplookup.NewLiveService(detailsSettings{}, nil)}}}
			r := httptest.NewRequest(http.MethodGet, "/api/v1/ip-lookup/details?ip=1.1.1.1", nil)
			r.Header.Set("Accept", accept)
			r = r.WithContext(context.WithValue(r.Context(), userContextKey, model.User{ID: "member", OnboardingState: "complete"}))
			w := &detailsRecorder{ResponseRecorder: httptest.NewRecorder()}
			s.ipLookupDetails(w, r)
			if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" || time.Until(w.deadline) < 60*time.Second {
				t.Fatalf("invalid status/cache/deadline: %d %v", w.Code, w.deadline)
			}
			if accept == "application/json" {
				var data iplookup.LiveDetails
				if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil || data.IP != "1.1.1.1" {
					t.Fatal("JSON compatibility lost")
				}
				return
			}
			if w.Header().Get("Content-Type") != accept || !w.Flushed || w.Header().Get("X-Accel-Buffering") != "no" {
				t.Fatal("stream was not flushed")
			}
			last := ""
			sections := 0
			for _, frame := range strings.Split(w.Body.String(), "\n\n") {
				if frame == "" {
					continue
				}
				var event iplookup.LiveEvent
				if err := json.Unmarshal([]byte(strings.TrimPrefix(frame, "data: ")), &event); err != nil {
					t.Fatal(err)
				}
				if event.Type == "section" {
					sections++
				}
				if event.Type == "progress" && event.Progress.Completed >= event.Progress.Total {
					t.Fatal("premature completion")
				}
				last = event.Type
			}
			if last != "complete" || sections < 6 {
				t.Fatal("missing sections or terminal event")
			}
		})
	}
}
