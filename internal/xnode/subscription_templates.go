package xnode

import (
	"context"
	"encoding/json"
	"maps"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const SubscriptionTemplatesKey = "xnode.subscription_templates"

// SubscriptionTemplateConfig persists administrator documents and references,
// never generated member configurations or copies of upstream user datasets.
type SubscriptionTemplateConfig struct {
	UserAgentRules []SubscriptionUserAgentRule  `json:"userAgentRules"`
	Revision       int64                        `json:"revision"`
	Templates      []subscription.Template      `json:"templates"`
	Defaults       map[string]string            `json:"defaults"`
	Assignments    map[string]map[string]string `json:"assignments"`
	Formats        map[string]bool              `json:"formats"`
}

func DefaultSubscriptionTemplates() SubscriptionTemplateConfig {
	result := SubscriptionTemplateConfig{UserAgentRules: []SubscriptionUserAgentRule{}, Templates: []subscription.Template{}, Defaults: map[string]string{}, Assignments: map[string]map[string]string{}, Formats: map[string]bool{"base64": true, "native": true, "xray": true, "sing-box": true, "clash": true, "stash": true}}
	for _, format := range []string{"xray", "sing-box", "clash", "stash"} {
		item := subscription.DefaultTemplate(format)
		result.Templates = append(result.Templates, item)
		result.Defaults[format] = item.ID
	}
	return result
}

func DecodeSubscriptionTemplates(raw string) (SubscriptionTemplateConfig, error) {
	if strings.TrimSpace(raw) == "" {
		return DefaultSubscriptionTemplates(), nil
	}
	var result SubscriptionTemplateConfig
	if len(raw) > 4<<20 || wire.Decode([]byte(raw), &result) != nil || result.Revision < 0 || len(result.Templates) > 64 || len(result.Assignments) > 10000 {
		return result, ErrInvalid
	}
	if result.Templates == nil {
		result.Templates = []subscription.Template{}
	}
	if result.Defaults == nil {
		result.Defaults = map[string]string{}
	}
	if result.Assignments == nil {
		result.Assignments = map[string]map[string]string{}
	}
	if result.Formats == nil {
		result.Formats = DefaultSubscriptionTemplates().Formats
	}
	if _, exists := result.Formats["base64"]; !exists {
		result.Formats["base64"] = result.Formats["native"]
	}
	if _, exists := result.Formats["stash"]; !exists {
		result.Formats["stash"] = true
	}
	if result.UserAgentRules == nil {
		result.UserAgentRules = []SubscriptionUserAgentRule{}
	}
	return result, nil
}

func (s *Service) SubscriptionTemplates(ctx context.Context) (SubscriptionTemplateConfig, error) {
	raw, err := s.settings.Optional(ctx, SubscriptionTemplatesKey)
	if err != nil {
		return SubscriptionTemplateConfig{}, err
	}
	return DecodeSubscriptionTemplates(raw)
}

func (s *Service) SaveSubscriptionTemplates(ctx context.Context, actor string, next SubscriptionTemplateConfig) (SubscriptionTemplateConfig, error) {
	s.templateMu.Lock()
	defer s.templateMu.Unlock()
	previous, err := s.SubscriptionTemplates(ctx)
	if err != nil {
		return next, err
	}
	if next.Revision != previous.Revision || next.Revision >= 9007199254740991 {
		return next, ErrConflict
	}
	if err = validateTemplateReferences(next); err != nil {
		return next, err
	}
	for _, item := range next.Templates {
		if _, validation, err := s.PreviewSubscriptionTemplate(item); err != nil || !validation.Valid {
			return next, ErrInvalid
		}
	}
	for user, refs := range next.Assignments {
		if len(refs) == 0 {
			delete(next.Assignments, user)
			continue
		}
		if maps.Equal(refs, previous.Assignments[user]) {
			continue
		}
		if _, err = s.repo.XNodeIdentityByUser(ctx, user); err != nil {
			return next, err
		}
	}
	next.Revision++
	raw, err := json.Marshal(next)
	if err != nil {
		return next, err
	}
	if len(raw) > 4<<20 {
		return next, ErrInvalid
	}
	writer, ok := s.settings.(interface {
		PutXNodeSubscriptionTemplates(context.Context, string, string) error
	})
	if !ok {
		return next, ErrUnavailable
	}
	if err = writer.PutXNodeSubscriptionTemplates(ctx, actor, string(raw)); err != nil {
		return next, err
	}
	return next, nil
}

func validateTemplateReferences(config SubscriptionTemplateConfig) error {
	if config.Templates == nil || config.Defaults == nil || config.Assignments == nil || config.Formats == nil {
		return ErrInvalid
	}
	if len(config.Templates) > 64 || len(config.Assignments) > 10000 || config.Revision < 0 {
		return ErrInvalid
	}
	items := map[string]string{}
	for _, item := range config.Templates {
		if !memberTagPattern.MatchString(item.ID) || strings.TrimSpace(item.Name) == "" || len(item.Name) > 128 || items[item.ID] != "" {
			return ErrInvalid
		}
		items[item.ID] = item.Format
	}
	validate := func(refs map[string]string) error {
		for format, id := range refs {
			if !templateFormat(format) || id != "" && items[id] != format {
				return ErrInvalid
			}
		}
		return nil
	}
	if validate(config.Defaults) != nil {
		return ErrInvalid
	}
	for user, refs := range config.Assignments {
		if len(user) == 0 || len(user) > 128 || validate(refs) != nil {
			return ErrInvalid
		}
	}
	for format := range config.Formats {
		if format != "native" && format != "base64" && !templateFormat(format) {
			return ErrInvalid
		}
	}
	return validateUserAgentRules(config)
}

func (s *Service) PreviewSubscriptionTemplate(item subscription.Template) ([]byte, ValidationResult, error) {
	raw, err := subscription.PreviewTemplate(item)
	if err != nil {
		return nil, ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID", Detail: validationDetail(err)}, nil
	}
	result := ValidationResult{Valid: true}
	if item.Format == "xray" {
		var configs []json.RawMessage
		if json.Unmarshal(raw, &configs) != nil || len(configs) != 1 {
			return nil, ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}, nil
		}
		var config map[string]json.RawMessage
		if json.Unmarshal(configs[0], &config) != nil || config == nil {
			return nil, ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}, nil
		}
		// Xray subscription clients use remarks as a display name. It is not a
		// server-core configuration field, so omit it only from native validation.
		delete(config, "remarks")
		validationDocument, err := json.Marshal(config)
		if err != nil {
			return nil, ValidationResult{ErrorCode: "XNODE_CONFIG_INVALID"}, nil
		}
		result = ValidateClientConfigurationDocument(validationDocument)
	}
	return raw, result, nil
}
