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
	// Write implies read only within the explicitly granted resource.
	if resource != "admin" && resource != "xnode" {
		return false
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
	// The generic database editor and full backups contain both commerce and
	// native control-plane state. A general administrator token must not use
	// these routes to bypass the explicit X-Node resource boundary.
	if resource == "database" || strings.HasPrefix(resource, "database/") || resource == "backups" || strings.HasPrefix(resource, "backups/") {
		parts := strings.Split(resource, "/")
		if method == http.MethodPost && len(parts) == 4 && parts[0] == "database" && parts[1] == "tables" && parts[3] == "query" {
			write = false
		}
		return p.Allows("admin", write) && p.Allows("xnode", write)
	}
	if strings.HasPrefix(resource, "operations/") && !write {
		return p.Allows("admin", false) || p.Allows("xnode", false)
	}
	if resource == "xnode" || strings.HasPrefix(resource, "xnode/") || resource == "hourly-policy" || strings.HasPrefix(resource, "hourly-policy/") || resource == "hourly-offers" || strings.HasPrefix(resource, "hourly-offers/") {
		return p.Allows("xnode", write)
	}
	return p.Allows("admin", write)
}

func (p Principal) AllowsOperation(kind string) bool {
	// Native operations keep their namespace even when new kinds are added.
	// They must never fall back to the general administrator scope.
	if strings.HasPrefix(kind, "xnode_") {
		return p.Allows("xnode", false)
	}
	return p.Allows("admin", false)
}
