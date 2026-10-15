package xnodemigration

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/google/uuid"
	"strings"
)

func (s *Service) importEndpoints(ctx context.Context, snapshot Snapshot, bindings map[string]map[string]string) error {
	existing, err := s.repo.XNodeEndpoints(ctx)
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, endpoint := range existing {
		ids[endpoint.ID] = true
	}
	for _, raw := range snapshot.Hosts {
		var host struct {
			ID          string   `json:"uuid"`
			Name        string   `json:"remark"`
			Address     string   `json:"address"`
			Port        int      `json:"port"`
			Nodes       []string `json:"nodes"`
			Disabled    bool     `json:"isDisabled"`
			Hidden      bool     `json:"isHidden"`
			SNI         *string  `json:"sni"`
			HTTPHost    *string  `json:"host"`
			Path        *string  `json:"path"`
			ALPN        *string  `json:"alpn"`
			Fingerprint *string  `json:"fingerprint"`
			Security    string   `json:"securityLayer"`
			Excluded    []string `json:"excludedInternalSquads"`
			Inbound     struct {
				ID *string `json:"configProfileInboundUuid"`
			} `json:"inbound"`
		}
		if json.Unmarshal(raw, &host) != nil || host.ID == "" || host.Inbound.ID == nil {
			return xnode.ErrInvalid
		}
		for _, node := range hostBindingNodes(host.Nodes, *host.Inbound.ID, bindings) {
			tag := bindings[node][*host.Inbound.ID]
			if tag == "" {
				continue
			}
			id := uuid.NewSHA1(uuid.NameSpaceURL, []byte("vansail-xnode:"+host.ID+":"+node)).String()
			if ids[id] {
				continue
			}
			overrides := map[string]any{"legacyHost": json.RawMessage(raw), "excludedGroups": host.Excluded}
			stream := map[string]any{}
			if host.Security == "NONE" {
				stream["security"] = "none"
			}
			if host.Security == "TLS" {
				stream["security"] = "tls"
			}
			if host.SNI != nil || host.ALPN != nil || host.Fingerprint != nil {
				security := map[string]any{}
				if host.SNI != nil {
					security["serverName"] = *host.SNI
				}
				if host.ALPN != nil {
					security["alpn"] = strings.Split(*host.ALPN, ",")
				}
				if host.Fingerprint != nil {
					security["fingerprint"] = *host.Fingerprint
				}
				overrides["securityOptions"] = security
			}
			transport := map[string]any{}
			if host.Path != nil {
				transport["path"] = *host.Path
			}
			if host.HTTPHost != nil {
				transport["host"] = *host.HTTPHost
			}
			if len(transport) > 0 {
				overrides["transportOptions"] = transport
			}
			if len(stream) > 0 {
				overrides["streamSettings"] = stream
			}
			payload, err := json.Marshal(overrides)
			if err != nil {
				return err
			}
			cipher, err := s.vault.Encrypt("xnode.endpoint."+id, string(payload))
			if err != nil {
				return err
			}
			if err = s.repo.SaveXNodeEndpoint(ctx, xnode.Endpoint{ID: id, NodeID: node, InboundTag: tag, Name: host.Name, Address: host.Address, Port: host.Port, Enabled: !host.Disabled, Hidden: host.Hidden, OverridesCipher: cipher}); err != nil {
				return err
			}
			ids[id] = true
		}
	}
	return nil
}
