package httpapi

import (
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http/httptest"
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
