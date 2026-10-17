package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"
	"regexp"
	"strings"
)

var managedGeoSiteSelector = regexp.MustCompile(`^geosite:[A-Za-z0-9_!.-]+(?:@[A-Za-z0-9_!.-]+)*$`)

func DecodeRoutingTemplates(raw string) (RoutingTemplateConfig, error) {
	config := RoutingTemplateConfig{Templates: []RoutingTemplate{}}
	if raw != "" && json.Unmarshal([]byte(raw), &config) != nil {
		return config, ErrInvalid
	}
	if config.Templates == nil {
		config.Templates = []RoutingTemplate{}
	}
	if config.Revision < 0 || len(config.Templates) > 64 {
		return config, ErrInvalid
	}
	ids := map[string]bool{}
	for _, t := range config.Templates {
		if !memberTagPattern.MatchString(t.ID) || ids[t.ID] || strings.TrimSpace(t.Name) == "" || len(t.Name) > 128 {
			return config, ErrInvalid
		}
		ids[t.ID] = true
		// Geosite identities are expanded only by the queued master resolver. Other
		// matcher syntax uses exactly the production graph compiler here.
		_, err := routingmatch.Build([]routingmatch.Rule{{ID: t.ID, From: "source", To: "target", Match: t.Match}}, func(selector string) ([]string, error) {
			if len(selector) > 128 || !managedGeoSiteSelector.MatchString(selector) {
				return nil, ErrInvalid
			}
			return []string{"domain:validation.invalid"}, nil
		})
		if err != nil {
			return config, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
	}
	return config, nil
}
func (s *Service) RoutingTemplateConfig(ctx context.Context) (RoutingTemplateConfig, error) {
	raw, err := s.settings.Optional(ctx, RoutingTemplatesKey)
	if err != nil {
		return RoutingTemplateConfig{}, err
	}
	return DecodeRoutingTemplates(raw)
}
func (s *Service) SaveRoutingTemplates(ctx context.Context, actor string, config RoutingTemplateConfig) (RoutingTemplateConfig, error) {
	if config.Templates == nil {
		config.Templates = []RoutingTemplate{}
	}
	s.templateMu.Lock()
	defer s.templateMu.Unlock()
	current, err := s.RoutingTemplateConfig(ctx)
	if err != nil {
		return current, err
	}
	if current.Revision != config.Revision {
		return current, ErrConflict
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return config, err
	}
	if _, err = DecodeRoutingTemplates(string(raw)); err != nil {
		return config, err
	}
	writer, ok := s.settings.(interface {
		PutXNodeRoutingTemplates(context.Context, string, string) error
	})
	if !ok {
		return config, ErrUnavailable
	}
	config.Revision++
	raw, err = json.Marshal(config)
	if err != nil {
		return config, err
	}
	if err = writer.PutXNodeRoutingTemplates(ctx, actor, string(raw)); err != nil {
		return config, err
	}
	return config, nil
}
