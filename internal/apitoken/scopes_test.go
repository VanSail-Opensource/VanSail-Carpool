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
		{"POST", "/api/v1/admin/xnode/migration/import", false},
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
	for _, scopes := range [][]string{nil, {"admin:root"}, {"admin:write", "admin:write"}} {
		if _, err := normalizeScopes(scopes); err == nil {
			t.Fatalf("invalid scopes accepted: %v", scopes)
		}
	}
}
