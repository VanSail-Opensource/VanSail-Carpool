package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNodeWriteRejectsRetiredMultiplier(t *testing.T) {
	for _, input := range []string{`{"multiplier":null}`, `{"multiplier":"1"}`, `{"multiplier":0}`, `{"multiplier":-1}`, `{"multiplier":1e99}`, `{"multiplier":[2]}`, `{"Multiplier":1}`, `{"unknown":true}`, `{"agentId":"forged"}`, `{"desiredRevision":10}`, `{"acceptedRevision":10}`, `{"appliedRevision":10}`, `{"createdAt":"2026-11-13T00:00:00Z"}`, `{"updatedAt":"2026-11-13T00:00:00Z"}`, `{"retiredAt":null}`, `{"removingAt":null}`} {
		t.Run(input, func(t *testing.T) {
			var request struct {
				Node nodeWrite `json:"node"`
			}
			r := httptest.NewRequest("POST", "/api/v1/admin/xnode/nodes", strings.NewReader(`{"node":`+input+`}`))
			if err := decodeJSON(httptest.NewRecorder(), r, &request); err == nil {
				t.Fatal("retired or unknown input was silently accepted", input)
			}
		})
	}
	var request struct {
		Node nodeWrite `json:"node"`
	}
	r := httptest.NewRequest("POST", "/api/v1/admin/xnode/nodes", strings.NewReader(`{"node":{"name":"Current node","configId":"config","enabled":true}}`))
	if err := decodeJSON(httptest.NewRecorder(), r, &request); err != nil || request.Node.Name != "Current node" || !request.Node.Enabled {
		t.Fatal("current node contract was rejected", request, err)
	}
}
