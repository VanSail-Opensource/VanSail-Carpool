package apitoken

import "testing"

func TestHourlyPolicyAndOffersKeepResourceScope(t *testing.T) {
	for _, scope := range []string{"admin:read", "admin:write", "xnode:read", "xnode:write"} {
		p := Principal{Item: Token{Scopes: []string{scope}}}
		for _, path := range []string{"/api/v1/admin/hourly-policy", "/api/v1/admin/hourly-offers", "/api/v1/admin/hourly-offers/squad"} {
			for _, method := range []string{"GET", "PUT"} {
				want := scope == "xnode:write" || method == "GET" && scope == "xnode:read"
				if p.AllowsRequest(method, path) != want {
					t.Errorf("%s %s %s did not enforce native scope", scope, method, path)
				}
			}
		}
		if p.AllowsRequest("POST", "/api/v1/hourly/activate") || p.AllowsRequest("GET", "/api/v1/hourly") {
			t.Fatal("administrator API key entered owner commerce", scope)
		}
		if p.AllowsOperation("xnode_hourly_activate") != (scope == "xnode:read" || scope == "xnode:write") {
			t.Fatal("hourly receipt crossed resource boundary", scope)
		}
	}
}
