package subscription

import (
	"regexp"
	"strconv"
	"strings"
)

// Upstream documents dot-separated keys and numeric array indices. Bracketed
// numeric indices are equivalent; exotic expression paths are explicitly rejected.
var mapperPathPattern = regexp.MustCompile(`^[A-Za-z0-9_$-]+(?:\.[A-Za-z0-9_$-]+|\[[0-9]+\])*$`)

func mapperSegments(path string) ([]string, error) {
	if len(path) > 512 || !mapperPathPattern.MatchString(path) {
		return nil, mapperError()
	}
	path = strings.ReplaceAll(strings.ReplaceAll(path, "[", "."), "]", "")
	parts := strings.Split(path, ".")
	if len(parts) > 32 {
		return nil, mapperError()
	}
	for _, part := range parts {
		if part == "__proto__" || part == "prototype" || part == "constructor" {
			return nil, mapperError()
		}
		if strings.IndexFunc(part, func(letter rune) bool { return letter < '0' || letter > '9' }) == -1 {
			if index, err := strconv.Atoi(part); err != nil || index > 1024 {
				return nil, mapperError()
			}
		}
	}
	return parts, nil
}

func mapperRead(value any, parts []string) (any, bool) {
	for _, part := range parts {
		switch item := value.(type) {
		case map[string]any:
			var found bool
			value, found = item[part]
			if !found {
				return nil, false
			}
		case []any:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(item) {
				return nil, false
			}
			value = item[index]
		default:
			return nil, false
		}
	}
	return value, true
}

// mapperUpdate retains array length on unset, producing JSON null for a hole as
// lodash.unset does. A primitive ancestor blocks writes instead of being replaced.
func mapperUpdate(node any, parts []string, value any, unset bool) (any, bool) {
	if len(parts) == 0 {
		return value, true
	}
	if node == nil {
		if unset {
			return node, false
		}
		if _, err := strconv.Atoi(parts[0]); err == nil {
			node = []any{}
		} else {
			node = map[string]any{}
		}
	}
	switch item := node.(type) {
	case map[string]any:
		if len(parts) == 1 {
			if unset {
				delete(item, parts[0])
			} else {
				item[parts[0]] = value
			}
			return item, true
		}
		child, changed := mapperUpdate(item[parts[0]], parts[1:], value, unset)
		if changed {
			item[parts[0]] = child
		}
		return item, changed
	case []any:
		index, err := strconv.Atoi(parts[0])
		if err != nil || index < 0 || index > 1024 || (unset && index >= len(item)) {
			return node, false
		}
		if index >= len(item) {
			item = append(item, make([]any, index-len(item)+1)...)
		}
		if len(parts) == 1 && unset {
			item[index] = nil
			return item, true
		}
		child, changed := mapperUpdate(item[index], parts[1:], value, unset)
		if changed {
			item[index] = child
		}
		return item, changed
	default:
		return node, false
	}
}
