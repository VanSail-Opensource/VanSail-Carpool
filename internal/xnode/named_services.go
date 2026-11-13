package xnode

import (
	"context"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// NamedService exposes access terms without credentials or probe identities.
type NamedService struct {
	Identity Identity      `json:"identity"`
	Access   ServiceAccess `json:"access"`
}

func (s *Service) NamedServices(ctx context.Context) ([]NamedService, error) {
	identities, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return nil, err
	}
	items := []NamedService{}
	for _, identity := range identities {
		if identity.MemberOwnerID() != "" {
			continue
		}
		access, err := s.ServiceAccess(ctx, identity)
		if err != nil {
			return nil, err
		}
		if access != nil {
			items = append(items, NamedService{Identity: identity, Access: *access})
		}
	}
	return items, nil
}

// SaveNamedService uses the same native quota periods as imported standalone
// accounts. Editing access never resets recorded usage or creates a principal.
func (s *Service) SaveNamedService(ctx context.Context, id, name string, access ServiceAccess) (NamedService, error) {
	return s.saveNamedService(ctx, id, name, access, "")
}

func (s *Service) saveNamedService(ctx context.Context, id, name string, access ServiceAccess, actor string) (NamedService, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(access.GroupIDs) == 0 || access.StartedAt.After(s.now()) || !access.ExpiresAt.After(s.now()) || !wire.ValidName(name) || strings.HasPrefix(name, "_") || strings.HasPrefix(name, paygPrefix) || strings.HasPrefix(name, memberProbePrefix) {
		return NamedService{}, ErrInvalid
	}
	var err error
	if id == "" {
		access, err = access.NamedWrite("")
		if err != nil {
			return NamedService{}, err
		}
	}
	groups, err := s.repo.XNodeGroups(ctx)
	if err != nil {
		return NamedService{}, err
	}
	known := map[string]bool{}
	for _, group := range groups {
		known[group.ID] = true
	}
	for _, group := range access.GroupIDs {
		if !known[group] {
			return NamedService{}, ErrInvalid
		}
	}
	identity := Identity{ServiceName: name, Username: name}
	var credentials Credentials
	if id != "" {
		identity, err = s.Identity(ctx, id)
		if err != nil {
			return NamedService{}, err
		}
		previous, e := s.ServiceAccess(ctx, identity)
		if e != nil {
			return NamedService{}, e
		}
		if previous == nil || identity.ServiceName != name {
			return NamedService{}, ErrInvalid
		}
		access, err = access.NamedWrite(previous.Strategy)
		if err != nil {
			return NamedService{}, err
		}
		credentials, err = loadCredentials(s.vault, identity)
		if err != nil {
			return NamedService{}, err
		}
	} else {
		identities, e := s.repo.XNodeIdentities(ctx)
		if e != nil {
			return NamedService{}, e
		}
		for _, item := range identities {
			if item.ServiceName == name {
				return NamedService{}, ErrConflict
			}
		}
		credentials, err = NewCredentials()
		if err != nil {
			return NamedService{}, err
		}
		if err = s.newSubscription(&identity); err != nil {
			return NamedService{}, err
		}
	}
	credentials.ServiceAccess = &access
	identity.CredentialsCipher, err = sealCredentials(s.vault, identity, credentials)
	if err != nil {
		return NamedService{}, err
	}
	identity, err = s.persistNamedService(ctx, actor, identity, access)
	return NamedService{Identity: identity, Access: access}, err
}
