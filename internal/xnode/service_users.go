package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"time"
)

func (s *Service) serviceUser(ctx context.Context, identity Identity, node Node, inbounds map[string]map[string]any) (wire.User, error) {
	bindings, err := s.repo.XNodeServiceBindings(ctx, identity.ProxyUserID)
	if err != nil {
		return wire.User{}, err
	}
	credentials, err := loadCredentials(s.vault, identity)
	if err != nil {
		return wire.User{}, err
	}
	user := wire.User{ID: int(identity.ProxyUserID), UUID: credentials.UUID, InboundTags: []string{}, Credentials: map[string]json.RawMessage{}, ConnectionLimit: 10, SpeedLimitMbps: 1000}
	var until time.Time
	for _, binding := range bindings {
		if binding.NodeID != node.ID || !binding.ValidUntil.After(s.now()) {
			continue
		}
		inbound := inbounds[binding.InboundTag]
		if inbound == nil {
			return user, ErrInvalid
		}
		credential, err := inboundCredential(inbound, credentials)
		if err != nil {
			return user, err
		}
		user.InboundTags = append(user.InboundTags, binding.InboundTag)
		user.Credentials[binding.InboundTag] = credential
		if until.IsZero() || binding.ValidUntil.Before(until) {
			until = binding.ValidUntil
		}
	}
	if !until.IsZero() {
		user.ValidUntil = &until
	}
	return user, nil
}

// AuthorizeService grants bounded probe access to exactly one endpoint.
func (s *Service) AuthorizeService(ctx context.Context, name string, endpoint Endpoint, until time.Time) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	identities, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return Identity{}, err
	}
	identity := Identity{ServiceName: name}
	found := false
	for _, item := range identities {
		if item.ServiceName == name {
			identity = item
			found = true
			break
		}
	}
	if !found {
		credentials, err := NewCredentials()
		if err != nil {
			return identity, err
		}
		identity.CredentialsCipher, err = sealCredentials(s.vault, identity, credentials)
		if err != nil {
			return identity, err
		}
		identity, err = s.repo.SaveXNodeIdentity(ctx, identity)
		if err != nil {
			return identity, err
		}
	}
	err = s.repo.SaveXNodeServiceBinding(ctx, ServiceBinding{ProxyUserID: identity.ProxyUserID, NodeID: endpoint.NodeID, InboundTag: endpoint.InboundTag, ValidUntil: until})
	return identity, err
}
