package routingmatch

import (
	"fmt"
	"regexp"
	"strings"
)

func parseAtom(raw string) (atom, error) {
	a := atom{kind: "keyword", value: raw}
	if len(raw) == 0 || len(raw) > 2048 {
		return a, fmt.Errorf("invalid domain selector")
	}
	for _, kind := range []string{"domain", "full", "keyword", "regexp"} {
		if v, ok := strings.CutPrefix(raw, kind+":"); ok {
			a.kind, a.value = kind, v
			break
		}
	}
	if sub, ok := strings.CutPrefix(raw, "dotless:"); ok {
		if strings.Contains(sub, ".") {
			return a, fmt.Errorf("dotless selector cannot contain a dot")
		}
		a.kind, a.value = "regexp", "^[^.]*"+sub+"[^.]*$"
	}
	if strings.HasPrefix(raw, "geoip:") || strings.HasPrefix(raw, "ext:") || strings.HasPrefix(raw, "geosite:") || a.value == "" {
		return a, fmt.Errorf("unsupported domain selector %s", raw)
	}
	if a.kind == "regexp" {
		var err error
		a.re, err = regexp.Compile(a.value)
		if err != nil {
			return a, fmt.Errorf("invalid Go expression: %w", err)
		}
	}
	return a, nil
}
func (a atom) matches(d string) bool {
	if a.kind == "all" {
		return true
	}
	if d == "" {
		return false
	}
	switch a.kind {
	case "full":
		return d == a.value
	case "domain":
		return d == a.value || strings.HasSuffix(d, "."+a.value)
	case "keyword":
		return strings.Contains(d, a.value)
	case "regexp":
		return a.re.MatchString(d)
	}
	return false
}

// relation returns an exact intersection for full/suffix sets. Unknown regex
// relationships stay unknown; a missing witness is never treated as disjoint.
func intersect(a, b atom) (atom, bool, bool) {
	if a.kind == "all" {
		return b, true, true
	}
	if b.kind == "all" {
		return a, true, true
	}
	if a.kind == "full" {
		return a, b.matches(a.value), true
	}
	if b.kind == "full" {
		return b, a.matches(b.value), true
	}
	if a.kind == "domain" && b.kind == "domain" {
		if a.matches(b.value) {
			return b, true, true
		}
		if b.matches(a.value) {
			return a, true, true
		}
		return atom{}, false, true
	}
	if a.kind == b.kind && a.value == b.value {
		return a, true, true
	}
	return a, true, false
}
func covers(a, b atom) bool {
	if a.kind == "all" {
		return true
	}
	if b.kind == "full" {
		return a.matches(b.value)
	}
	return a.kind == b.kind && a.value == b.value || a.kind == "domain" && b.kind == "domain" && a.matches(b.value)
}
func domainAnchors(positive []compiled, budget *int) ([]atom, bool) {
	anchors := []atom{{kind: "all"}}
	exact := true
	for _, c := range positive {
		if len(c.domains) == 0 {
			continue
		}
		next := []atom{}
		for _, a := range anchors {
			for _, b := range c.domains {
				*budget--
				if *budget < 0 {
					return nil, false
				}
				v, ok, known := intersect(a, b)
				exact = exact && known
				if ok {
					next = append(next, v)
				}
				if len(next) > 4096 {
					return nil, false
				}
			}
		}
		anchors = next
		if len(anchors) == 0 {
			return anchors, true
		}
	}
	return anchors, exact
}
func domainCovered(outer compiled, anchors []atom, budget *int) bool {
	if len(outer.domains) == 0 {
		return true
	}
	for _, anchor := range anchors {
		found := false
		for _, a := range outer.domains {
			*budget--
			if *budget < 0 {
				return false
			}
			if covers(a, anchor) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
