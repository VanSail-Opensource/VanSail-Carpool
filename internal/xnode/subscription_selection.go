package xnode

import (
	"errors"
	"regexp"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

type SubscriptionUserAgentRule struct {
	Pattern    string `json:"pattern"`
	TemplateID string `json:"templateId"`
}

type SubscriptionSelection struct {
	Format     string `json:"format"`
	TemplateID string `json:"templateId"`
	Source     string `json:"source"`
}

func templateFormat(format string) bool {
	return format == "xray" || format == "sing-box" || format == "clash" || format == "stash"
}

// DetectSubscriptionFormat is the built-in client detector. Unknown clients
// receive real Base64; the explicit native format remains plain share links.
func DetectSubscriptionFormat(agent string) string {
	agent = strings.ToLower(agent)
	switch {
	case strings.Contains(agent, "stash"):
		return "stash"
	case strings.Contains(agent, "sing-box"), strings.Contains(agent, "singbox"):
		return "sing-box"
	case strings.Contains(agent, "clash"), strings.Contains(agent, "mihomo"):
		return "clash"
	default:
		return "base64"
	}
}

func validateUserAgentRules(config SubscriptionTemplateConfig) error {
	if len(config.UserAgentRules) > 64 {
		return ErrInvalid
	}
	for _, rule := range config.UserAgentRules {
		if rule.Pattern == "" || len(rule.Pattern) > 2048 {
			return ErrInvalid
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return ErrInvalid
		}
		found := false
		for _, item := range config.Templates {
			if item.ID == rule.TemplateID && templateFormat(item.Format) {
				found = true
				break
			}
		}
		if !found {
			return ErrInvalid
		}
	}
	return nil
}

// SelectSubscription uses exactly the same Go RE2 matcher for previews and live
// requests. Ordered administrator matches carry an exact template reference.
func SelectSubscription(config SubscriptionTemplateConfig, explicit, agent string) (SubscriptionSelection, error) {
	if len(agent) > 8192 || validateUserAgentRules(config) != nil {
		return SubscriptionSelection{}, ErrInvalid
	}
	selection := SubscriptionSelection{Format: explicit, Source: "explicit"}
	if explicit == "" {
		for _, rule := range config.UserAgentRules {
			matched, err := regexp.MatchString(rule.Pattern, agent)
			if err != nil {
				return selection, ErrInvalid
			}
			if matched {
				for _, item := range config.Templates {
					if item.ID == rule.TemplateID {
						selection = SubscriptionSelection{Format: item.Format, TemplateID: item.ID, Source: "regex"}
						break
					}
				}
				break
			}
		}
		if selection.Format == "" {
			selection.Format = DetectSubscriptionFormat(agent)
			selection.Source = "builtin"
			if selection.Format == "base64" {
				selection.Source = "fallback"
			}
		}
	}
	if !config.Formats[selection.Format] {
		return selection, &PolicyError{"SUBSCRIPTION_FORMAT_DISABLED", ErrInvalid}
	}
	return selection, nil
}

func PreviewSubscriptionSelection(config SubscriptionTemplateConfig, explicit, agent string) (SubscriptionSelection, ValidationResult) {
	if len(config.UserAgentRules) > 64 {
		return SubscriptionSelection{}, ValidationResult{ErrorCode: "XNODE_INPUT_INVALID"}
	}
	for _, rule := range config.UserAgentRules {
		if len(rule.Pattern) > 2048 {
			return SubscriptionSelection{}, ValidationResult{ErrorCode: "XNODE_INPUT_INVALID"}
		}
		if _, err := regexp.Compile(rule.Pattern); err != nil {
			return SubscriptionSelection{}, ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID", Detail: validationDetail(err)}
		}
	}
	if err := validateTemplateReferences(config); err != nil {
		return SubscriptionSelection{}, ValidationResult{ErrorCode: "XNODE_INPUT_INVALID"}
	}
	selection, err := SelectSubscription(config, explicit, agent)
	if err == nil {
		_, err = renderSubscriptionSelection(config, "", selection, subscription.PreviewProxies())
	}
	if err != nil {
		var policy *PolicyError
		if errors.As(err, &policy) {
			return selection, ValidationResult{ErrorCode: policy.Code}
		}
		return selection, ValidationResult{ErrorCode: "XNODE_INPUT_INVALID"}
	}
	return selection, ValidationResult{Valid: true}
}
