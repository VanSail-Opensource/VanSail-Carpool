package httpapi

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSubscriptionLandingNegotiatesBrowserAndClientContent(t *testing.T) {
	for _, tc := range []struct {
		name, accept, agent, query, format string
		html                               bool
	}{
		{"browser", "text/html,application/xhtml+xml,*/*;q=0.8", "Mozilla/5.0", "", "base64", true},
		{"native", "*/*", "v2rayN", "", "base64", false},
		{"singbox", "*/*", "sing-box/1.14", "", "sing-box", false},
		{"mihomo", "*/*", "mihomo/1.19", "", "clash", false},
		{"explicit", "text/html", "Mozilla/5.0", "?format=xray", "xray", false},
		{"html disabled", "application/json,text/html;q=0.0", "", "", "base64", false},
		{"json preferred", "application/json,text/html;q=0.5", "", "", "base64", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/subscription/token"+tc.query, nil)
			r.Header.Set("Accept", tc.accept)
			r.Header.Set("User-Agent", tc.agent)
			if got := subscriptionContentFormat(r); got != tc.format {
				t.Fatalf("format=%s, want %s", got, tc.format)
			}
			if got := subscriptionWantsHTML(r); got != tc.html {
				t.Fatalf("html=%v, want %v", got, tc.html)
			}
		})
	}
	s := &Server{deps: Dependencies{Static: fstest.MapFS{"subscription.html": &fstest.MapFile{Data: []byte("public subscription page")}}}}
	r := httptest.NewRequest("GET", "/subscription/token", nil)
	r.Header.Set("Accept", "text/html")
	w := httptest.NewRecorder()
	s.subscriptionLanding(w, r)
	if w.Code != 200 || w.Body.String() != "public subscription page" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("browser landing=%d,%s,%v", w.Code, w.Body.String(), w.Header())
	}
}
