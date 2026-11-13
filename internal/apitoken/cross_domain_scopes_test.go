package apitoken

import (
	"net/http"
	"testing"
)

func TestDatabaseAndBackupsRequireEveryResourceScope(t *testing.T) {
	for _, test := range []struct {
		name        string
		scopes      []string
		read, write bool
	}{
		{"admin read", []string{"admin:read"}, false, false},
		{"admin write", []string{"admin:write"}, false, false},
		{"native read", []string{"xnode:read"}, false, false},
		{"native write", []string{"xnode:write"}, false, false},
		{"both read", []string{"admin:read", "xnode:read"}, true, false},
		{"admin write native read", []string{"admin:write", "xnode:read"}, true, false},
		{"admin read native write", []string{"admin:read", "xnode:write"}, true, false},
		{"both write", []string{"admin:write", "xnode:write"}, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			principal := Principal{Item: Token{Scopes: test.scopes}}
			for _, request := range []struct {
				method, path string
				want         bool
			}{
				{http.MethodGet, "/api/v1/admin/database/tables", test.read},
				{http.MethodGet, "/api/v1/admin/database/tables/xnode_nodes/rows", test.read},
				{http.MethodPost, "/api/v1/admin/database/tables/xnode_nodes/query", test.read},
				{http.MethodPut, "/api/v1/admin/database/tables/xnode_nodes/rows", test.write},
				{http.MethodPost, "/api/v1/admin/database/mutations/review", test.write},
				{http.MethodPost, "/api/v1/admin/database/mutations", test.write},
				{http.MethodPost, "/api/v1/admin/database/mutations/query", test.write},
				{http.MethodPost, "/api/v1/admin/database/tables/xnode_nodes/extra/query", test.write},
				{http.MethodGet, "/api/v1/admin/backups", test.read},
				{http.MethodGet, "/api/v1/admin/backups/backup/download", test.read},
				{http.MethodPost, "/api/v1/admin/backups", test.write},
				{http.MethodDelete, "/api/v1/admin/backups/backup", test.write},
			} {
				if got := principal.AllowsRequest(request.method, request.path); got != request.want {
					t.Errorf("%s %s allowed=%v want %v", request.method, request.path, got, request.want)
				}
			}
		})
	}
}
