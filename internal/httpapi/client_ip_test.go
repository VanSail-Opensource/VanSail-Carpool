package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"testing"
)

func TestClientIPUsesOnlyConfiguredProxySuffix(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24"), netip.MustParsePrefix("2001:db8:1::/48")}
	for _, test := range []struct{ name, peer, xff, real, want string }{
		{"direct ignores headers", "198.51.100.2:1234", "203.0.113.99", "203.0.113.88", "198.51.100.2"},
		{"trusted suffix", "192.0.2.1:1234", "203.0.113.99, 198.51.100.2,192.0.2.2", "203.0.113.88", "198.51.100.2"},
		{"malformed untrusted prefix ignored", "192.0.2.1:1234", "not-an-ip, 198.51.100.2", "", "198.51.100.2"},
		{"malformed trusted suffix refused", "192.0.2.1:1234", "198.51.100.2,not-an-ip", "203.0.113.88", "192.0.2.1"},
		{"single real IP fallback", "192.0.2.1:1234", "", "198.51.100.2", "198.51.100.2"},
		{"ambiguous real IP refused", "192.0.2.1:1234", "", "198.51.100.2,203.0.113.2", "192.0.2.1"},
		{"IPv6 canonical", "[2001:db8:1::1]:1234", "2001:0db8:2:0:0:0:0:7", "", "2001:db8:2::7"},
		{"mapped peer canonical", "[::ffff:192.0.2.1]:1234", "::ffff:198.51.100.2", "", "198.51.100.2"},
		{"header zone refused", "192.0.2.1:1234", "fe80::1%client", "", "192.0.2.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "/", nil)
			request.RemoteAddr = test.peer
			if test.xff != "" {
				request.Header.Set("X-Forwarded-For", test.xff)
			}
			if test.real != "" {
				request.Header.Set("X-Real-IP", test.real)
			}
			got := resolveClientIP(request, trusted)
			if got.String() != test.want || request.RemoteAddr != test.peer {
				t.Fatal(got, request.RemoteAddr)
			}
		})
	}
}

func TestForwardingCannotBypassOrAmplifyAuthenticationBuckets(t *testing.T) {
	for _, trusted := range []bool{false, true} {
		server := &Server{authLimiter: newAuthLimiter()}
		prefixes := []netip.Prefix{}
		peer := "198.51.100.2:1234"
		if trusted {
			prefixes = append(prefixes, netip.MustParsePrefix("192.0.2.1/32"))
			peer = "192.0.2.1:1234"
		}
		expectedPeer := peer
		handler := clientIPMiddleware(prefixes)(server.limitAuthentication(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RemoteAddr != expectedPeer {
				t.Error("transport peer was overwritten")
			}
			w.WriteHeader(204)
		})))
		for i := range authLimitCapacity + 1 {
			request := httptest.NewRequest("POST", "/api/v1/auth/telegram", nil)
			request.RemoteAddr = peer
			// A proxy appends the real visitor. The visitor may supply arbitrary left prefixes.
			xff := "not-an-ip-" + strconv.Itoa(i) + ",198.51.100.2"
			request.Header.Set("X-Forwarded-For", xff)
			request.Header.Set("X-Real-IP", "203.0.113."+strconv.Itoa(i+1))
			request.Header.Set("CF-Connecting-IP", "203.0.113.99")
			request.Header.Set("Forwarded", "for=203.0.113.98")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			want := 204
			if i == authLimitCapacity {
				want = 429
			}
			if response.Code != want {
				t.Fatalf("trusted %v attempt %d status %d want %d", trusted, i, response.Code, want)
			}
		}
		request := httptest.NewRequest("POST", "/api/v1/auth/telegram", nil)
		request.RemoteAddr = "198.51.100.3:2345"
		if trusted {
			request.RemoteAddr = peer
			request.Header.Set("X-Forwarded-For", "not-an-ip,198.51.100.3")
		}
		expectedPeer = request.RemoteAddr
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 204 {
			t.Fatal("another visitor inherited the attacker bucket", response.Code)
		}
	}
}

func TestForwardedHeaderLinesAreReadFromTrustedRightEdge(t *testing.T) {
	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "192.0.2.1:443"
	request.Header["X-Forwarded-For"] = []string{"203.0.113.99,198.51.100.2", "192.0.2.8"}
	trusted := []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")}
	if got := resolveClientIP(request, trusted).String(); got != "198.51.100.2" {
		t.Fatal(got)
	}
	if got := resolveClientIP(request, nil).String(); got != "192.0.2.1" {
		t.Fatal("empty trust config accepted headers", got)
	}
	request.Header["X-Forwarded-For"] = []string{""}
	request.Header.Set("X-Real-IP", "203.0.113.99")
	if got := resolveClientIP(request, trusted).String(); got != "192.0.2.1" {
		t.Fatal("empty chain fell back to a conflicting header", got)
	}
}
