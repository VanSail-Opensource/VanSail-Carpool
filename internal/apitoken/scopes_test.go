package apitoken

import "testing"

func TestScopedControlSeparatesMigrationCommerceAndCredentials(t *testing.T) {
	p := Principal{Item: Token{Scopes: []string{"xnode:write"}}}
	for _, entry := range []struct {
		method, path string
		allow        bool
	}{
		{"POST", "/api/v1/admin/xnode/nodes", true},
		{"GET", "/api/v1/admin/xnode/nodes", true},

		{"GET", "/api/v1/admin/payments", false},
		{"GET", "/api/v1/me", false},
		{"GET", "/api/v1/admin/xnode/../settings", false},
		{"GET", "/api/v1/admin//xnode/nodes", false},
	} {
		if actual := p.AllowsRequest(entry.method, entry.path); actual != entry.allow {
			t.Errorf("%s %s=%v", entry.method, entry.path, actual)
		}
	}
	p.Item.Scopes = []string{"admin:write"}
	if p.AllowsRequest("POST", "/api/v1/admin/api-tokens") || p.AllowsRequest("DELETE", "/api/v1/admin/api-tokens/owned") {
		t.Fatal("bearer can manage credential endpoints")
	}
	p.Item.Scopes = []string{"xnode:read"}
	if !p.AllowsRequest("POST", "/api/v1/admin/xnode/firewall/records/search") || p.AllowsRequest("POST", "/api/v1/admin/xnode/firewall/exemptions") {
		t.Fatal("read-only firewall search scope permits a mutation or denies search")
	}
	for _, scopes := range [][]string{nil, {"admin:root"}, {"admin:write", "admin:write"}} {
		if _, err := normalizeScopes(scopes); err == nil {
			t.Fatalf("invalid scopes accepted: %v", scopes)
		}
	}
}

func TestAdministratorScopesDoNotImplyNativeControl(t *testing.T) {
	for _, scope := range []string{"admin:read", "admin:write"} {
		p := Principal{Item: Token{Scopes: []string{scope}}}
		for _, entry := range [][2]string{
			{"GET", "/api/v1/admin/xnode/nodes"},
			{"GET", "/api/v1/admin/xnode/service-identities/identity/subscription"},
			{"GET", "/api/v1/admin/xnode/operations/receipt"},
			{"POST", "/api/v1/admin/xnode/nodes/node/enrollment"},
			{"POST", "/api/v1/admin/xnode/nodes/node/commands"},
		} {
			if p.AllowsRequest(entry[0], entry[1]) {
				t.Errorf("%s crossed native boundary: %v", scope, entry)
			}
		}
		for _, kind := range []string{"xnode_command", "xnode_routing", "xnode_future_operation"} {
			if p.AllowsOperation(kind) {
				t.Errorf("%s exposed native receipt %s", scope, kind)
			}
		}
		if !p.AllowsRequest("GET", "/api/v1/admin/settings") || p.Allows("unknown", false) {
			t.Fatal("resource scope changed", scope)
		}
		if p.AllowsRequest("PUT", "/api/v1/admin/settings") != (scope == "admin:write") {
			t.Fatal("write distinction changed", scope)
		}
	}
	p := Principal{Item: Token{Scopes: []string{"admin:write", "xnode:read"}}}
	if !p.AllowsRequest("GET", "/api/v1/admin/xnode/nodes") || !p.AllowsOperation("xnode_command") || p.AllowsRequest("POST", "/api/v1/admin/xnode/nodes/node/enrollment") {
		t.Fatal("read scope inherited write")
	}
}
