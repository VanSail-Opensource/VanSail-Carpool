package xnode

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestPublicAssetRedirectPolicy(t *testing.T) {
	previous := make([]*http.Request, 3)
	for index := range previous {
		previous[index] = &http.Request{}
	}

	request := &http.Request{URL: &url.URL{Scheme: "https", Host: "cdn.example", Path: "/geoip.dat"}}
	if err := validatePublicAssetRedirect(request, previous); err != nil {
		t.Fatalf("valid HTTPS redirect was rejected: %v", err)
	}
	if err := validatePublicAssetRedirect(request, append(previous, &http.Request{})); !errors.Is(err, ErrInvalid) {
		t.Fatalf("redirect chain over the limit = %v", err)
	}
	for _, target := range []string{
		"http://cdn.example/geoip.dat",
		"https://user:secret@cdn.example/geoip.dat",
		"https://cdn.example/geoip.dat#fragment",
	} {
		parsed, err := url.Parse(target)
		if err != nil {
			t.Fatal(err)
		}
		if err = validatePublicAssetRedirect(&http.Request{URL: parsed}, nil); !errors.Is(err, ErrInvalid) {
			t.Errorf("unsafe redirect %q = %v", target, err)
		}
	}
}

func TestAssetRequestFailureKeepsRedirectRejectionDistinct(t *testing.T) {
	invalid := assetRequestFailure("GeoIP asset", ErrInvalid)
	if !errors.Is(invalid, ErrInvalid) || errors.Is(invalid, ErrUnavailable) {
		t.Fatalf("invalid redirect classification = %v", invalid)
	}
	unavailable := assetRequestFailure("GeoSite asset", errors.New("connection refused"))
	if !errors.Is(unavailable, ErrUnavailable) {
		t.Fatalf("network failure classification = %v", unavailable)
	}
}

func TestEnabledGeoIPIsNotProbedTwiceAfterValidatedDownload(t *testing.T) {
	config := DefaultGlobal()
	config.GeoIPURL = "https://assets.example/geoip.dat"
	config.GeoSiteURL = "https://assets.example/geosite.dat"
	config.SourceGeoIP.Enabled = true
	if requiresAssetLinkProbe(config.GeoIPURL, config) {
		t.Fatal("validated GeoIP asset would receive a duplicate range probe")
	}
	if !requiresAssetLinkProbe(config.GeoSiteURL, config) {
		t.Fatal("GeoSite link skipped its independent link check")
	}
	config.SourceGeoIP.Enabled = false
	if !requiresAssetLinkProbe(config.GeoIPURL, config) {
		t.Fatal("unvalidated GeoIP link skipped link validation")
	}
}
