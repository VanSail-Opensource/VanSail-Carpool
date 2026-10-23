package connectivity

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDecodeConfigBoundariesAndSecrets(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{"defaults", "", true},
		{"configured", `{"proxyUserId":42,"scheduledEnabled":true}`, true},
		{"minimum", `{"proxyUserId":42,"intervalSeconds":60,"timeoutSeconds":1}`, true},
		{"maximum", `{"proxyUserId":42,"intervalSeconds":86400,"timeoutSeconds":60}`, true},
		{"missing_account", `{"scheduledEnabled":true}`, false},
		{"negative_account", `{"proxyUserId":-1}`, false},
		{"short_interval", `{"intervalSeconds":59}`, false},
		{"long_interval", `{"intervalSeconds":86401}`, false},
		{"zero_timeout", `{"timeoutSeconds":0}`, false},
		{"long_timeout", `{"timeoutSeconds":61}`, false},
		{"zero_retries", `{"maxRetries":0}`, true},
		{"maximum_retries", `{"maxRetries":10,"retryIntervalSeconds":60}`, true},
		{"negative_retries", `{"maxRetries":-1}`, false},
		{"excess_retries", `{"maxRetries":11}`, false},
		{"fractional_retries", `{"maxRetries":1.5}`, false},
		{"zero_retry_interval", `{"retryIntervalSeconds":0}`, false},
		{"negative_retry_interval", `{"retryIntervalSeconds":-1}`, false},
		{"long_retry_interval", `{"retryIntervalSeconds":61}`, false},
		{"fractional_retry_interval", `{"retryIntervalSeconds":1.5}`, false},
		{"http", `{"probeUrl":"http://example.com/"}`, false},
		{"credentials", `{"probeUrl":"https://secret:password@example.com/"}`, false},
		{"relative", `{"probeUrl":"/generate_204"}`, false},
		{"unknown", `{"password":"secret"}`, false},
		{"null", `null`, false},
		{"trailing_object", `{} {}`, false},
		{"wrong_type", `{"proxyUserId":"42"}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := DecodeConfig(test.value)
			if (err == nil) != test.valid {
				t.Fatalf("valid=%t, config=%+v error=%v", test.valid, cfg, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatal("configuration error exposed a credential")
			}
		})
	}
	cfg := DefaultConfig()
	if cfg.ScheduledEnabled || cfg.ProxyUserID != 0 || cfg.IntervalSeconds != 300 || cfg.TimeoutSeconds != 15 ||
		cfg.ProbeURL != "https://cp.cloudflare.com/generate_204" || cfg.MaxRetries != 10 || cfg.RetryIntervalSeconds != 1 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestConfigHashSeparatesChangedProbeIdentity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ProxyUserID = 42
	first := ConfigHash(cfg)
	if len(first) != 64 || ConfigHash(cfg) != first {
		t.Fatal("configuration fingerprint is unstable")
	}
	for _, change := range []func(*Config){
		func(c *Config) { c.ProxyUserID++ },
		func(c *Config) { c.ProbeURL = "https://example.com/generate_204" },
		func(c *Config) { c.TimeoutSeconds++ },
		func(c *Config) { c.MaxRetries-- },
		func(c *Config) { c.RetryIntervalSeconds++ },
		func(c *Config) { c.IntervalSeconds++ },
		func(c *Config) { c.ScheduledEnabled = true },
	} {
		copy := cfg
		change(&copy)
		if ConfigHash(copy) == first {
			t.Fatal("changed settings reused old latest-result identity")
		}
	}
}

func TestDecodeConfigPreservesLegacyDefaultsAndExplicitZero(t *testing.T) {
	legacy, err := DecodeConfig(`{"proxyUserId":42,"intervalSeconds":300,"timeoutSeconds":15,"probeUrl":"https://example.com"}`)
	if err != nil || legacy.MaxRetries != 10 || legacy.RetryIntervalSeconds != 1 {
		t.Fatalf("legacy retry defaults: %+v %v", legacy, err)
	}
	disabled, err := DecodeConfig(`{"maxRetries":0}`)
	if err != nil || disabled.MaxRetries != 0 || disabled.RetryIntervalSeconds != 1 {
		t.Fatalf("explicit zero lost: %+v %v", disabled, err)
	}
}

func TestErrorCodeNeverExposesProviderSecrets(t *testing.T) {
	for _, err := range []error{
		errors.New("https://token:password@example.com/private"),
		&CodeError{Code: "secret credential"},
		&CodeError{Code: "CONNECTIVITY_ACCOUNT_UNAVAILABLE", Err: errors.New("secret token")},
	} {
		code := ErrorCode(err)
		if !safeErrorCode.MatchString(code) || strings.Contains(code, "secret") {
			t.Fatalf("unsafe public error code: %q", code)
		}
	}
	if ErrorCode(context.Canceled) != "CONNECTIVITY_INTERRUPTED" || ErrorCode(context.DeadlineExceeded) != "CONNECTIVITY_PROBE_TIMEOUT" {
		t.Fatal("cancellation classification was lost")
	}
}
