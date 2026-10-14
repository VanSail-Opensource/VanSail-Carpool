package iplookup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// IDs defines the built-in providers and the legacy default execution order.
var IDs = func() []string {
	ids := []string{}
	for _, provider := range Registry {
		ids = append(ids, provider.ID)
	}
	return ids
}()

// DefaultGeolocationOrder resolves equal or missing location quality.
func DefaultGeolocationOrder() []string {
	return []string{"ip2location", "ipapi", "maxmind", "scamalytics", "abuseipdb"}
}

// CodeError exposes only a stable member-safe error identifier.
type CodeError struct{ Code string }

func (e *CodeError) Error() string { return e.Code }

// CredentialKey binds encryption to the exact provider setting.
func CredentialKey(id string) string { return "ip_lookup." + id + ".credential" }

// DefaultConfig leaves prices unconfigured and all providers disabled.
func DefaultConfig() Config {
	c := Config{Providers: []ProviderConfig{}, GeolocationOrder: DefaultGeolocationOrder(), Retry: nil, ProxySources: DefaultProxySources(), DNSResolvers: defaultResolvers(), ProbePorts: []int{443, 4430, 8443}}
	for _, id := range IDs {
		c.Providers = append(c.Providers, ProviderConfig{ID: id})
	}
	return c
}

// DecodeConfig validates both orders and appends omitted providers disabled.
func DecodeConfig(raw string) (Config, error) {
	if raw == "" {
		return DefaultConfig(), nil
	}
	var c Config
	if json.Unmarshal([]byte(raw), &c) != nil {
		return c, &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
	}
	for _, fee := range []string{c.LookupFeeTXB, c.RefreshFeeTXB} {
		if fee != "" {
			if amount, err := model.ParseTXBMajor(fee); err != nil || amount < 0 || amount > 1_000_000_000_000 {
				return c, &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
			}
		}
	}
	byID := map[string]ProviderConfig{}
	for _, p := range c.Providers {
		known := false
		for _, id := range IDs {
			if p.ID == id {
				known = true
			}
		}
		if _, duplicate := byID[p.ID]; duplicate || !known {
			return c, &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
		}
		if p.ID == "maxmind" && p.Enabled {
			if p.AccountID == "" || strings.Trim(p.AccountID, "0123456789") != "" {
				return c, &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
			}
		}
		if p.ID == "scamalytics" && p.Enabled {
			if p.AccountID == "" || len(p.AccountID) > 80 || strings.Trim(p.AccountID, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-") != "" {
				return c, &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
			}
		}
		byID[p.ID] = p
	}
	for _, id := range IDs {
		if _, present := byID[id]; !present {
			c.Providers = append(c.Providers, ProviderConfig{ID: id})
		}
	}
	if err := validateRetryPolicy(c.Retry); err != nil {
		return c, err
	}
	if err := normalizeEnhancedConfig(&c); err != nil {
		return c, err
	}
	if c.GeolocationOrder == nil {
		c.GeolocationOrder = DefaultGeolocationOrder()
	}
	if len(c.GeolocationOrder) != len(DefaultGeolocationOrder()) {
		return c, &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
	}
	geoIDs := map[string]bool{}
	for _, id := range DefaultGeolocationOrder() {
		geoIDs[id] = true
	}
	for _, id := range c.GeolocationOrder {
		if !geoIDs[id] {
			return c, &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
		}
		delete(geoIDs, id)
	}
	if c.Enabled {
		if c.LookupFeeTXB == "" || c.RefreshFeeTXB == "" {
			return c, &CodeError{"IP_LOOKUP_CONFIGURATION_REQUIRED"}
		}
		enabled := false
		for _, p := range c.Providers {
			enabled = enabled || p.Enabled
		}
		if !enabled {
			return c, &CodeError{"IP_LOOKUP_CONFIGURATION_REQUIRED"}
		}
	}
	return c, nil
}

// ConfigHash freezes public configuration in a signed quote.
func ConfigHash(c Config) string {
	b, _ := json.Marshal(c)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// validateRetryPolicy bounds explicit repair settings; nil stays nil so
// deployments without the fields keep the enabled defaults.
func validateRetryPolicy(p *RetryPolicy) error {
	if p == nil {
		return nil
	}
	if p.MaxAttempts < 0 || p.MaxAttempts > 10 {
		return &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
	}
	if p.DelaySeconds < 0 || p.DelaySeconds > 86400 {
		return &CodeError{"IP_LOOKUP_INVALID_CONFIG"}
	}
	return nil
}

// ErrorCode prevents HTTP clients and queue logs from leaking credential-bearing URLs.
func ErrorCode(err error) string {
	var coded *CodeError
	if errors.As(err, &coded) {
		return coded.Code
	}
	return "IP_LOOKUP_FAILED"
}
