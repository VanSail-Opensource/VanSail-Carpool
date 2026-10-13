package httpapi

import (
	"net/http"
	"net/url"
	"strings"
)

// legacyMiniAppFrame permits only the neutral dispatcher to forward old Telegram
// iframe launches into the restricted access panel. It never authenticates a user.
func legacyMiniAppFrame(r *http.Request) bool {
	if r.URL.Path != "/admin" && !strings.HasPrefix(r.URL.Path, "/admin/") {
		return false
	}
	if r.URL.Path == "/admin/access" || r.URL.Path == "/admin/login" {
		return false
	}
	if r.Header.Get("Sec-Fetch-Dest") != "iframe" {
		return false
	}
	referrer, err := url.Parse(r.Referer())
	if err != nil || referrer.Scheme != "https" {
		return false
	}
	host := strings.ToLower(referrer.Hostname())
	return host == "web.telegram.org" || strings.HasSuffix(host, ".telegram.org")
}
