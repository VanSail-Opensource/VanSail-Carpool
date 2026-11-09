package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNodeWriteRejectsRetiredAndUnknownSettings(t *testing.T) {
	for _, test := range []struct {
		name, settings string
		valid          bool
	}{
		{"current", `{"dns":{"mode":"xray"},"firewall":{"enabled":true}}`, true},
		{"retired bandwidth", `{"bandwidth":{"normalMbps":100}}`, false},
		{"null retired bandwidth", `{"bandwidth":null}`, false},
		{"unknown setting", `{"future":true}`, false},
		{"unknown nested setting", `{"dns":{"future":true}}`, false},
		{"wrong known type", `{"firewall":{"enabled":"true"}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/xnode/nodes/node", strings.NewReader(`{"node":{},"settings":`+test.settings+`}`))
			var input struct {
				Node     xnode.Node         `json:"node"`
				Settings xnode.NodeSettings `json:"settings"`
			}
			err := decodeJSON(httptest.NewRecorder(), request, &input)
			if (err == nil) != test.valid {
				t.Fatalf("request accepted=%v, want %v: %v", err == nil, test.valid, err)
			}
		})
	}
}
