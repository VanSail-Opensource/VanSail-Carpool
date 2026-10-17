package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"time"
)

func (s *Service) serviceUser(ctx context.Context, identity Identity, node Node, inbounds map[string]map[string]any) (wire.User, error) {
	credentials, err := loadCredentials(s.vault, identity)
	if err != nil {
		return wire.User{}, err
	}
	user := wire.User{ID: int(identity.ProxyUserID), UUID: credentials.UUID, InboundTags: []string{}, Credentials: map[string]json.RawMessage{}, ConnectionLimit: 10, SpeedLimitMbps: 1000}
	if identity.AccessDisabled {
		return user, nil
	}
	if access := credentials.ServiceAccess; access != nil {
		period, err := s.servicePeriod(ctx, identity, access)
		if err != nil {
			return user, err
		}
		if period == nil {
			return user, nil
		}
		user.AccountingPeriod = period.ID
		user.ConnectionLimit, user.SpeedLimitMbps = 0, 0
	}
	bindings, err := s.serviceBindings(ctx, identity, credentials.ServiceAccess)
	if err != nil {
		return user, err
	}
	var until time.Time
	seen := map[string]bool{}
	for _, binding := range bindings {
		if binding.NodeID != node.ID || !binding.ValidUntil.After(s.now()) || seen[binding.InboundTag] {
			continue
		}
		seen[binding.InboundTag] = true
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
