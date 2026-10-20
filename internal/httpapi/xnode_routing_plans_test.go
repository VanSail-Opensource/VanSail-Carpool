package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutingErrorsExposeOnlyBoundedOwnerRuleReferences(t *testing.T) {
	s := &Server{}
	request := httptest.NewRequest("POST", "/api/v1/xnode/routing/preview", nil)
	response := httptest.NewRecorder()
	s.xnodeRoutingError(response, request, fmt.Errorf("%w: %w", xnode.ErrInvalid, &routingmatch.Diagnostic{Code: "routing_loop", Rules: []string{"ab", "ba"}}))
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 422 || body.Code != "routing_loop" || body.Details["rules"] != "ab,ba" {
		t.Fatalf("missing actionable rule diagnostic: %s", response.Body.String())
	}
}

func TestRoutingPolicyHTTPMappingsAndSafeWarnings(t *testing.T) {
	for _, tc := range []struct {
		code    string
		invalid bool
	}{{"XNODE_GEOSITE_REQUIRED", true}, {"XNODE_GEOSITE_UNAVAILABLE", false}, {"XNODE_DOMAIN_SELECTOR_INVALID", true}} {
		t.Run(tc.code, func(t *testing.T) {
			var log bytes.Buffer
			s := &Server{deps: Dependencies{Logger: slog.New(slog.NewTextHandler(&log, nil))}}
			cause := xnode.ErrUnavailable
			status := 503
			if tc.invalid {
				cause, status = xnode.ErrInvalid, 422
			}
			err := errors.Join(&xnode.PolicyError{Code: tc.code}, cause, errors.New("https://private.example/asset?token=secret"))
			response := httptest.NewRecorder()
			s.xnodeRoutingError(response, httptest.NewRequest("POST", "/api/v1/xnode/routing/preview", nil), err)
			var body apiError
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if response.Code != status || body.Code != tc.code || log.Len() == 0 || strings.Contains(log.String(), "secret") {
				t.Fatal(response.Code, body, log.String())
			}
		})
	}
}

func TestRoutingCapabilityFailureRequestsAgentUpgrade(t *testing.T) {
	s := &Server{deps: Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	request := httptest.NewRequest("PUT", "/api/v1/xnode/routing", nil)
	response := httptest.NewRecorder()
	s.xnodeError(response, request, fmt.Errorf("node edge: %w", &xnode.CapabilityError{Capability: "transit_credentials_v1"}))
	var body apiError
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 409 || body.Code != "XNODE_UPGRADE_REQUIRED" {
		t.Fatalf("capability failure = %d %s", response.Code, response.Body.String())
	}
}
