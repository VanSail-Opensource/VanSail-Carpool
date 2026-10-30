package apitoken

import (
	"net/http"
	"slices"
	"sort"
	"strings"
)

var scopeNames = []string{"admin:read", "admin:write", "xnode:read", "xnode:write"}

func normalizeScopes(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > len(scopeNames) {
		return nil, ErrInvalid
	}
	result := []string{}
	for _, value := range values {
		if !slices.Contains(scopeNames, value) || slices.Contains(result, value) {
			return nil, ErrInvalid
		}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func (p Principal) Allows(resource string, write bool) bool {
	if slices.Contains(p.Item.Scopes, "admin:write") {
		return true
	}
	if !write && slices.Contains(p.Item.Scopes, "admin:read") {
		return true
	}
	return slices.Contains(p.Item.Scopes, resource+":write") || !write && slices.Contains(p.Item.Scopes, resource+":read")
}

func (p Principal) AllowsRequest(method, path string) bool {
	const root = "/api/v1/admin/"
	if !strings.HasPrefix(path, root) || strings.Contains(path, "\\") || strings.Contains(path, "//") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	resource := strings.TrimPrefix(path, root)
	// Credentials are issued/revoked only in the signed browser console. The
	// obsolete detector is deliberately outside automation's control plane.
	if resource == "api-tokens" || strings.HasPrefix(resource, "api-tokens/") || resource == "abuse" || strings.HasPrefix(resource, "abuse/") {
		return false
	}
	write := method != http.MethodGet && method != http.MethodHead
	if method == http.MethodPost && resource == "xnode/firewall/records/search" {
		write = false
	}
	if strings.HasPrefix(resource, "operations/") && !write {
		return p.Allows("admin", false) || p.Allows("xnode", false)
	}
	if resource == "xnode" || strings.HasPrefix(resource, "xnode/") {
		return p.Allows("xnode", write)
	}
	return p.Allows("admin", write)
}

func (p Principal) AllowsOperation(kind string) bool {
	switch kind {
	case "xnode_command", "xnode_routing":
		return p.Allows("xnode", false)
	default:
		return p.Allows("admin", false)
	}
}
