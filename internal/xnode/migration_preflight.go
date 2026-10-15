package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

// VerifyClientEndpoints checks every enabled future endpoint even if the node
// currently has no paid members. Temporary credentials are never stored/granted.
func (s *Service) VerifyClientEndpoints(ctx context.Context, node string) error {
	credentials, err := NewCredentials()
	if err != nil {
		return err
	}
	identity := Identity{ServiceName: "migration-preflight"}
	identity.CredentialsCipher, err = sealCredentials(s.vault, identity, credentials)
	if err != nil {
		return err
	}
	endpoints, err := s.Endpoints(ctx)
	if err != nil {
		return err
	}
	for _, endpoint := range endpoints {
		if endpoint.NodeID != node || !endpoint.Enabled {
			continue
		}
		if _, err := filterEndpointGroups(nil, endpoint.Overrides); err != nil {
			return err
		}
		proxy, err := s.endpointProxy(ctx, identity, endpoint)
		if err != nil {
			return err
		}
		if _, err = subscription.RenderXray([]subscription.Proxy{proxy}); err != nil {
			return err
		}
	}
	return nil
}

func validShadowsocksMethods(native map[string]any) bool {
	inbounds, _ := native["inbounds"].([]any)
	for _, raw := range inbounds {
		inbound, _ := raw.(map[string]any)
		if inbound["protocol"] != "shadowsocks" {
			continue
		}
		settings, _ := inbound["settings"].(map[string]any)
		method := settings["method"]
		if method != "chacha20-ietf-poly1305" && method != "2022-blake3-aes-128-gcm" && method != "2022-blake3-aes-256-gcm" {
			return false
		}
	}
	return true
}
