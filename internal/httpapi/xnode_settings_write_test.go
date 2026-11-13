package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNodeSettingsWriteRejectsRetiredAndPrivateFields(t *testing.T) {
	for _, input := range []string{
		`{"dns":{"mode":"xray"}}`, `{"dns":{"mode":"smartdns"}}`, `{"dns":{"upstreams":[]}}`,
		`{"legacyDnsLoopback":true}`, `{"nativeDnsFallbackServers":["127.0.0.1"]}`,
		`{"ddnsBindings":{"wan":{"ipv4HostId":"host"}}}`, `{"ddnsBindings":{"wan":{"ipv6HostId":"host"}}}`,
		`{"ddnsBindings":{"wan":{"unknown":true}}}`, `{"ddnsBindings":{"wan":{"configuredBy":"forged"}}}`,
	} {
		var request struct {
			Settings xnode.NodeSettings `json:"settings"`
		}
		r := httptest.NewRequest("POST", "/api/v1/admin/xnode/nodes", strings.NewReader(`{"settings":`+input+`}`))
		if err := decodeJSON(httptest.NewRecorder(), r, &request); err == nil {
			t.Fatal("retired or private settings were accepted", input)
		}
	}
}

func TestNodeSettingsWritePreservesOmittedAndExplicitEmptyBindings(t *testing.T) {
	for _, input := range []struct {
		settings string
		omitted  bool
	}{
		{`{"dns":{"redirect":true}}`, true},
		{`{"dns":{"redirect":true},"ddnsBindings":{}}`, false},
		{`{"dns":{"redirect":true},"ddnsBindings":{"wan":{"ipv4HostIds":[],"ipv6HostIds":["host"]}}}`, false},
	} {
		var request struct {
			Settings xnode.NodeSettings `json:"settings"`
		}
		r := httptest.NewRequest("POST", "/api/v1/admin/xnode/nodes", strings.NewReader(`{"settings":`+input.settings+`}`))
		if err := decodeJSON(httptest.NewRecorder(), r, &request); err != nil || !request.Settings.DNS.Redirect || (request.Settings.DDNSBindings == nil) != input.omitted {
			t.Fatal("current node settings changed omission semantics", request, err)
		}
	}
}
