package xnode

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// RepairImportedTemplates upgrades endpoints imported before client-template
// conversion existed. Explicit administrator overrides are never overwritten.
func (s *Service) RepairImportedTemplates(ctx context.Context, node string, templates map[string]json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	endpoints, err := s.repo.XNodeEndpoints(ctx)
	if err != nil {
		return err
	}
	for _, endpoint := range endpoints {
		if endpoint.NodeID != node {
			continue
		}
		var overrides map[string]any
		if err = s.open("xnode.endpoint."+endpoint.ID, endpoint.OverridesCipher, &overrides); err != nil {
			return err
		}
		if overrides["xrayTemplate"] != nil {
			continue
		}
		host, _ := overrides["legacyHost"].(map[string]any)
		id, _ := host["xrayJsonTemplateUuid"].(string)
		if id == "" {
			continue
		}
		document, err := subscription.ConvertXrayTemplate(templates[id])
		if err != nil {
			return ErrInvalid
		}
		overrides["xrayTemplate"] = document
		raw, err := json.Marshal(overrides)
		if err != nil {
			return err
		}
		endpoint.OverridesCipher, err = s.vault.Encrypt("xnode.endpoint."+endpoint.ID, string(raw))
		if err != nil {
			return err
		}
		if err = s.repo.SaveXNodeEndpoint(ctx, endpoint); err != nil {
			return err
		}
	}
	return nil
}
