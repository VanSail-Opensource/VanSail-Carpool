package xnode

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func (s *Service) SaveNode(ctx context.Context, node Node, settings NodeSettings) (Node, error) {
	return s.saveNode(ctx, node, settings, "")
}

func (s *Service) saveNode(ctx context.Context, node Node, settings NodeSettings, actor string) (Node, error) {
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
	return s.persistNodeDDNS(ctx, node, settings.DDNSBindings, actor)
}
