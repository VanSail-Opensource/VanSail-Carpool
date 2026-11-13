package admin

import (
	"context"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func init() {
	settingDefinitions[xnode.GlobalSettingKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := xnode.DecodeGlobal(raw); return err }}
	settingDefinitions[xnode.SubscriptionTemplatesKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := xnode.DecodeSubscriptionTemplates(raw); return err }}
	settingDefinitions[xnode.RoutingTemplatesKey] = SettingDefinition{Secret: true, Validate: func(raw string) error { _, err := xnode.DecodeRoutingTemplates(raw); return err }}
}

func (s *SettingsService) PutXNodeRoutingTemplates(ctx context.Context, actor, raw string) error {
	return s.put(ctx, actor, xnode.RoutingTemplatesKey, raw)
}

// PutXNodeSubscriptionTemplates accepts only the validated fleet service path.
func (s *SettingsService) PutXNodeSubscriptionTemplates(ctx context.Context, actor, raw string) error {
	return s.put(ctx, actor, xnode.SubscriptionTemplatesKey, raw)
}

// PutXNodeGlobalAtVersion accepts only the fleet service's validated candidate.
// The database retains current authority and compile inputs through the commit.
func (s *SettingsService) PutXNodeGlobalAtVersion(ctx context.Context, actor, raw string, version int64, until time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return xnode.ErrInvalid
	}
	if _, err := xnode.DecodeGlobal(raw); err != nil {
		return err
	}
	repo, ok := s.repository.(interface {
		PutAdminXNodeGlobalAtVersion(context.Context, string, string, int64, time.Time) error
	})
	if !ok {
		return xnode.ErrUnavailable
	}
	cipher, err := s.vault.Encrypt(xnode.GlobalSettingKey, raw)
	if err != nil {
		return err
	}
	return repo.PutAdminXNodeGlobalAtVersion(ctx, actor, cipher, version, until)
}
