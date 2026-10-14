package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"math"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) SaveNode(ctx context.Context, node Node, settings NodeSettings) (Node, error) {
	normalizeNodeSettings(&settings)
	if strings.TrimSpace(node.Name) == "" || len(node.Name) > 128 || node.ConfigID == "" || math.IsNaN(node.Multiplier) || math.IsInf(node.Multiplier, 0) || node.Multiplier <= 0 || node.Multiplier > 1000 {
		return node, ErrInvalid
	}
	if len(node.CountryCode) != 0 && len(node.CountryCode) != 2 {
		return node, ErrInvalid
	}
	node.CountryCode = strings.ToUpper(node.CountryCode)
	if _, err := s.repo.XNodeConfiguration(ctx, node.ConfigID); err != nil {
		return node, err
	}
	if node.ID == "" {
		var err error
		node.ID, err = ids.New()
		if err != nil {
			return node, err
		}
	}
	node.MultiplierFP = int64(math.Round(node.Multiplier * float64(MultiplierScale)))
	global, err := s.Global(ctx)
	if err != nil {
		return node, err
	}
	validation := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{}, Providers: global.Providers, Certificates: settings.Certificates, InboundCertificates: settings.InboundCertificates, DDNS: settings.DDNS, DNS: settings.DNS, Firewall: settings.Firewall}
	if err = validation.Validate(); err != nil {
		return node, ErrInvalid
	}
	node.SettingsCipher, err = s.seal("xnode.node."+node.ID, settings)
	if err != nil {
		return node, err
	}
	previous, previousErr := s.repo.XNodeNode(ctx, node.ID)
	if previousErr != nil && !isMissing(previousErr) {
		return node, previousErr
	}
	if previousErr == nil && previous.AgentID != "" && previous.MultiplierFP != node.MultiplierFP {
		if s.master == nil {
			return node, ErrUnavailable
		}
		barrier, err := s.master.Flush(ctx, node.ID, 0, false)
		if err != nil {
			return node, err
		}
		if err = s.repo.SetXNodeRate(ctx, node.ID, barrier.AgentID, barrier.Sequence, node.MultiplierFP); err != nil {
			return node, err
		}
	}
	if err = s.repo.SaveXNodeNode(ctx, node); err != nil {
		return node, err
	}
	return s.repo.XNodeNode(ctx, node.ID)
}
func (s *Service) Enrollment(ctx context.Context, id string) (string, string, error) {
	if _, err := s.repo.XNodeNode(ctx, id); err != nil {
		return "", "", err
	}
	token, err := ids.Token(32)
	if err != nil {
		return "", "", err
	}
	hash := sha256.Sum256([]byte(token))
	if s.master != nil {
		err = s.master.rotateEnrollment(ctx, id, hash[:])
	} else {
		err = s.repo.RotateXNodeToken(ctx, id, hash[:])
	}
	if err != nil {
		return "", "", err
	}
	u := *s.publicURL
	u.Scheme = "wss"
	u.Path = "/v1/agents/connect"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), token, nil
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
		node, err := s.repo.XNodeNode(ctx, binding.NodeID)
		if err != nil {
			return group, err
		}
		profile, err := s.ResolveConfiguration(ctx, node.ConfigID)
		if err != nil {
			return group, err
		}
		var native struct {
			Inbounds []struct {
				Tag string `json:"tag"`
			} `json:"inbounds"`
		}
		if json.Unmarshal(profile.Document, &native) != nil {
			return group, ErrInvalid
		}
		exists := false
		for _, inbound := range native.Inbounds {
			if inbound.Tag == binding.InboundTag {
				exists = true
			}
		}
		key := binding.NodeID + ":" + binding.InboundTag
		if !exists || seen[key] {
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
	if strings.TrimSpace(item.Name) == "" || item.Address == "" || strings.ContainsAny(item.Address, "\x00\r\n /?#@") || item.Port < 1 || item.Port > 65535 || !wire.ValidName(item.InboundTag) {
		return item, ErrInvalid
	}
	if _, err := s.repo.XNodeNode(ctx, item.NodeID); err != nil {
		return item, err
	}
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
	var err error
	item.OverridesCipher, err = s.vault.Encrypt("xnode.endpoint."+item.ID, string(item.Overrides))
	if err != nil {
		return item, err
	}
	return item, s.repo.SaveXNodeEndpoint(ctx, item)
}
