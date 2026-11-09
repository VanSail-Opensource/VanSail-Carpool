package xnode

import (
	"context"
	"encoding/json"
	"errors"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// EndpointMetadata describes the resolved inbound, never an account or secret.
type EndpointMetadata struct {
	Protocol  string `json:"protocol"`
	Transport string `json:"transport"`
	Security  string `json:"security"`
}

func (s *Service) endpointInbound(ctx context.Context, nodeID, tag string) (Node, NodeSettings, map[string]any, error) {
	node, settings, inbounds, err := s.endpointInbounds(ctx, nodeID)
	if err != nil {
		return node, settings, nil, err
	}
	for _, inbound := range inbounds {
		if inbound["tag"] == tag {
			return node, settings, inbound, nil
		}
	}
	return node, settings, nil, ErrInvalid
}

func (s *Service) endpointInbounds(ctx context.Context, nodeID string) (Node, NodeSettings, []map[string]any, error) {
	node, settings, err := s.Node(ctx, nodeID)
	if err != nil {
		return node, settings, nil, err
	}
	configuration, err := s.ResolveConfiguration(ctx, node.ConfigID)
	if err != nil {
		return node, settings, nil, err
	}
	var native map[string]any
	if json.Unmarshal(configuration.Document, &native) != nil {
		return node, settings, nil, ErrInvalid
	}
	// Match compilation's saved snippet expansion without changing the source
	// document returned by ResolveConfiguration or editing shared configuration.
	for _, id := range configuration.SnippetIDs {
		snippet, err := s.repo.XNodeSnippet(ctx, id)
		if err != nil {
			return node, settings, nil, err
		}
		var objects []any
		if err = s.open("xnode.snippet."+id, snippet.DocumentCipher, &objects); err != nil {
			return node, settings, nil, err
		}
		if err = appendSharedSnippet(native, snippet.Kind, objects); err != nil {
			return node, settings, nil, err
		}
	}
	inbounds, _ := native["inbounds"].([]any)
	result := []map[string]any{}
	for _, raw := range inbounds {
		inbound, _ := raw.(map[string]any)
		switch stringField(inbound, "protocol") {
		case "vless", "vmess", "trojan", "shadowsocks", "hysteria", "socks", "http", "mixed":
			result = append(result, wire.Clone(inbound))
		}
	}
	return node, settings, result, nil
}

func inboundMetadata(inbound map[string]any, settings NodeSettings, tag string) EndpointMetadata {
	stream, _ := inbound["streamSettings"].(map[string]any)
	result := EndpointMetadata{Protocol: stringField(inbound, "protocol"), Transport: stringField(stream, "network"), Security: stringField(stream, "security")}
	if result.Transport == "" || result.Transport == "raw" {
		result.Transport = "tcp"
	}
	if result.Security == "" {
		result.Security = "none"
	}
	if settings.InboundCertificates[tag] != "" {
		result.Security = "tls"
	}
	return result
}

// AdminEndpoints derives display metadata on demand. Member inventories retain
// their existing narrow read path without a second per-endpoint projection.
func (s *Service) AdminEndpoints(ctx context.Context) ([]Endpoint, error) {
	s = s.withCompileReads()
	items, err := s.Endpoints(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		_, settings, inbound, err := s.endpointInbound(ctx, items[i].NodeID, items[i].InboundTag)
		if err != nil && !isMissing(err) && !errors.Is(err, ErrInvalid) {
			return nil, err
		}
		metadata := EndpointMetadata{}
		if err == nil {
			metadata = inboundMetadata(inbound, settings, items[i].InboundTag)
		}
		items[i].Metadata = &metadata
	}
	return items, nil
}
