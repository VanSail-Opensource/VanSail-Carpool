package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// PolicyError exposes a stable safe code without disclosing asset URLs.
type PolicyError struct {
	Code  string
	cause error
}

func (e *PolicyError) Error() string { return e.Code }
func (e *PolicyError) Unwrap() error { return e.cause }

func (s *Service) validateGlobalPolicies(ctx context.Context, config GlobalConfig) error {
	for _, selector := range config.BlockedDomains {
		if strings.TrimSpace(selector) != selector {
			return &PolicyError{"XNODE_DOMAIN_SELECTOR_INVALID", ErrInvalid}
		}
	}
	base := wire.Config{Xray: json.RawMessage(`{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`), BlockedDomains: config.BlockedDomains}
	if !ValidateCompiled(base).Valid {
		return &PolicyError{"XNODE_DOMAIN_SELECTOR_INVALID", ErrInvalid}
	}
	selectors := []string{}
	for _, selector := range config.BlockedDomains {
		if strings.HasPrefix(selector, "geosite:") && !slices.Contains(selectors, selector) {
			selectors = append(selectors, selector)
		}
	}
	if len(selectors) > 0 {
		if config.GeoSiteURL == "" {
			return &PolicyError{"XNODE_GEOSITE_REQUIRED", ErrInvalid}
		}
		if _, err := s.downloadRoutingGeoSite(ctx, config, selectors); err != nil {
			code := "XNODE_GEOSITE_UNAVAILABLE"
			if errors.Is(err, ErrInvalid) {
				code = "XNODE_DOMAIN_SELECTOR_INVALID"
			}
			return &PolicyError{code, err}
		}
	}
	if config.SourceGeoIP.Enabled {
		if _, err := s.resolveGeoIPCached(ctx, config); err != nil {
			return geoIPPolicyFailure(err)
		}
	}
	return nil
}

func geoIPPolicyFailure(err error) error {
	code := "XNODE_GEOIP_UNAVAILABLE"
	if errors.Is(err, ErrInvalid) {
		code = "XNODE_GEOIP_INVALID"
	}
	return &PolicyError{code, err}
}
