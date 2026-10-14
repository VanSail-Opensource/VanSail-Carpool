package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/google/uuid"
	"strconv"
)

// ImportServiceIdentity is available only to explicitly mapped monitoring
// accounts. Service access is separately bounded to the imported group bindings.
func (s *Service) ImportServiceIdentity(ctx context.Context, name string, legacyID int64, uuidValue, trojan, chacha string) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !wire.ValidName(name) || legacyID <= 0 || legacyID >= 1<<40 || trojan == "" || chacha == "" {
		return Identity{}, ErrInvalid
	}
	parsed, err := uuid.Parse(uuidValue)
	if err != nil || parsed == uuid.Nil {
		return Identity{}, ErrInvalid
	}
	identities, err := s.repo.XNodeIdentities(ctx)
	if err != nil {
		return Identity{}, err
	}
	for _, item := range identities {
		if item.ServiceName != name && item.ProxyUserID != legacyID {
			continue
		}
		credentials, err := loadCredentials(s.vault, item)
		if err != nil {
			return item, err
		}
		if item.ServiceName != name || item.ProxyUserID != legacyID || credentials.UUID != uuidValue || credentials.Trojan != trojan || credentials.Shadowsocks != chacha {
			return item, ErrConflict
		}
		return item, nil
	}
	credentials, err := NewCredentials()
	if err != nil {
		return Identity{}, err
	}
	credentials.UUID, credentials.Trojan, credentials.Shadowsocks = uuidValue, trojan, chacha
	identity := Identity{ProxyUserID: legacyID, ServiceName: name, LegacyUserID: strconv.FormatInt(legacyID, 10)}
	identity.CredentialsCipher, err = sealCredentials(s.vault, identity, credentials)
	if err != nil {
		return identity, err
	}
	return s.repo.SaveXNodeIdentity(ctx, identity)
}
