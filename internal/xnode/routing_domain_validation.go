package xnode

import "github.com/xtls/xray-core/common/geodata"

// validateExpandedDomains runs native parsing and matcher compilation after the
// bounded category/attribute lookup. It needs no process-wide asset environment
// and catches malformed expressions inside an otherwise valid protobuf asset.
func validateExpandedDomains(selectors []string) error {
	rules, err := geodata.ParseDomainRules(selectors, geodata.Domain_Substr)
	if err != nil {
		return ErrInvalid
	}
	if _, err = geodata.DomainReg.BuildDomainMatcher(rules); err != nil {
		return ErrInvalid
	}
	return nil
}
