package xnode

import (
	"context"
	"errors"
	"testing"
)

func TestSharedDomainValidationWithoutEnrolledNodes(t *testing.T) {
	for _, test := range []struct{ selector, code string }{
		{"regexp:[", "XNODE_DOMAIN_SELECTOR_INVALID"},
		{"geosite:google", "XNODE_GEOSITE_REQUIRED"},
		{" domain:google.com", "XNODE_DOMAIN_SELECTOR_INVALID"},
	} {
		t.Run(test.selector, func(t *testing.T) {
			config := DefaultGlobal()
			config.BlockedDomains = []string{test.selector}
			err := (&Service{}).validateGlobalPolicies(context.Background(), config)
			var policy *PolicyError
			if !errors.As(err, &policy) || policy.Code != test.code {
				t.Fatalf("validation = %v", err)
			}
		})
	}
}
