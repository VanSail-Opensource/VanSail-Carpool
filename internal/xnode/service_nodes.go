package xnode

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) SaveNode(ctx context.Context, node Node, settings NodeSettings) (Node, error) {
	normalizeNodeSettings(&settings)
	var rangeErr error
	settings.Firewall, rangeErr = settings.Firewall.NormalizePortRanges()
	if rangeErr != nil {
		return node, ErrInvalid
	}
	settings.Firewall.Rules = nil
	settings.Firewall.BanSeconds = 0
	settings.Firewall.MasterControlled = false // negotiated per authenticated peer
	settings.Firewall.SourceGeoIP = nil        // only the master may resolve this projection
	if strings.TrimSpace(node.Name) == "" || len(node.Name) > 128 || node.ConfigID == "" {
		return node, ErrInvalid
	}
	if len(node.CountryCode) != 0 && len(node.CountryCode) != 2 {
		return node, ErrInvalid
	}
	node.CountryCode = strings.ToUpper(node.CountryCode)
	base, err := s.repo.XNodeConfiguration(ctx, node.ConfigID)
	if err != nil {
		return node, err
	}
	if node.ID == "" {
		var err error
		node.ID, err = ids.New()
		if err != nil {
			return node, err
		}
	}
	previous, previousErr := s.repo.XNodeNode(ctx, node.ID)
	if previousErr != nil && !isMissing(previousErr) {
		return node, previousErr
	}
	if previousErr == nil {
		if previous.RetiredAt != nil || previous.RemovingAt != nil {
			return node, ErrConflict
		}
		if err = s.preserveNodeDNS(previous, &settings); err != nil {
			return node, err
		}
	}
	node.Multiplier, node.MultiplierFP = 1, MultiplierScale
	global, err := s.Global(ctx)
	if err != nil {
		return node, err
	}
	if err = validateCertificateSettings(settings, global.Providers); err != nil {
		return node, err
	}
	if err = s.prepareDDNSSettings(ctx, node.ID, &settings); err != nil {
		return node, err
	}
	validation := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{}, Providers: global.Providers, Certificates: settings.Certificates, InboundCertificates: settings.InboundCertificates, DDNS: settings.DDNS, DNS: settings.DNS, Firewall: settings.Firewall}
	validation.DDNS = ddnsJobsForNode(node.ID, settings, global.Providers)
	if settings.DNS.Redirect {
		raw, e := s.vault.Decrypt("xnode.config."+base.ID, base.DocumentCipher)
		if e != nil {
			return node, e
		}
		validation.Xray, err = nodeDNSDocument(json.RawMessage(raw), settings)
		if err != nil {
			return node, err
		}
	}
	if err = validation.Validate(); err != nil {
		return node, ErrInvalid
	}
	stored := settings
	stored.DDNSBindings = nil
	node.SettingsCipher, err = s.seal("xnode.node."+node.ID, stored)
	if err != nil {
		return node, err
	}
	if err = s.persistNodeDDNS(ctx, node, settings.DDNSBindings); err != nil {
		return node, err
	}
	return s.repo.XNodeNode(ctx, node.ID)
}
func (s *Service) Groups(ctx context.Context) ([]Group, error) { return s.repo.XNodeGroups(ctx) }
func (s *Service) SaveGroup(ctx context.Context, group Group) (Group, error) {
	if strings.TrimSpace(group.Name) == "" || len(group.Name) > 128 {
		return group, ErrInvalid
	}
	if group.ID == "" {
		var err error
		group.ID, err = ids.New()
		if err != nil {
			return group, err
		}
	}
	seen := map[string]bool{}
	for _, binding := range group.Bindings {
		if !wire.ValidName(binding.InboundTag) {
			return group, ErrInvalid
		}
		if err := s.validateManagedBinding(ctx, binding.NodeID, binding.InboundTag); err != nil {
			return group, err
		}
		key := binding.NodeID + ":" + binding.InboundTag
		if seen[key] {
			return group, ErrInvalid
		}
		seen[key] = true
	}
	return group, s.repo.SaveXNodeGroup(ctx, group)
}
func (s *Service) Endpoints(ctx context.Context) ([]Endpoint, error) {
	items, err := s.repo.XNodeEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		raw, err := s.vault.Decrypt("xnode.endpoint."+items[i].ID, items[i].OverridesCipher)
		if err != nil {
			return nil, err
		}
		items[i].Overrides = json.RawMessage(raw)
	}
	return items, nil
}
func (s *Service) SaveEndpoint(ctx context.Context, item Endpoint) (Endpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview, err := s.PreviewEndpoint(ctx, item)
	if err != nil {
		return item, err
	}
	if !preview.Valid {
		return item, ErrInvalid
	}
	item.Metadata = &preview.Metadata
	if item.ID == "" {
		var err error
		item.ID, err = ids.New()
		if err != nil {
			return item, err
		}
	}
	if len(item.Overrides) == 0 {
		item.Overrides = json.RawMessage(`{}`)
	}
	var overrides map[string]any
	if json.Unmarshal(item.Overrides, &overrides) != nil {
		return item, ErrInvalid
	}
	if _, err := filterEndpointGroups(nil, item.Overrides); err != nil {
		return item, err
	}
	item.OverridesCipher, err = s.vault.Encrypt("xnode.endpoint."+item.ID, string(item.Overrides))
	if err != nil {
		return item, err
	}
	return item, s.repo.SaveXNodeEndpoint(ctx, item)
}
