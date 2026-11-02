package httpapi

import (
	"net/url"
	"strings"
)

func sameOrigin(actual, expected *url.URL) bool {
	if actual == nil || expected == nil || actual.IsAbs() == false || actual.User != nil ||
		actual.Path != "" || actual.RawQuery != "" || actual.Fragment != "" {
		return false
	}
	return strings.EqualFold(actual.Scheme, expected.Scheme) &&
		strings.EqualFold(actual.Hostname(), expected.Hostname()) &&
		effectivePort(actual) == effectivePort(expected)
}

func effectivePort(value *url.URL) string {
	if port := value.Port(); port != "" {
		return port
	}
	if strings.EqualFold(value.Scheme, "https") {
		return "443"
	}
	if strings.EqualFold(value.Scheme, "http") {
		return "80"
	}
	return ""
}
