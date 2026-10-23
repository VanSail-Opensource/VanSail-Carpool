package subscription

import "strings"

func tuningRestrictions(spec tuningSpec, format string) []string {
	result := []string{}
	for _, rule := range spec.Restrictions {
		if rule == "fixedOff" || spec.Mappings[format] != "" && (rule == "serverCapability" || strings.HasPrefix(rule, "singbox") && format == "sing-box" || strings.HasPrefix(rule, "stash") && format == "stash") {
			result = append(result, rule)
		}
	}
	return result
}
