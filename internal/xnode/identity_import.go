package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/google/uuid"
	"strconv"
)

// ImportIdentity is resumable and preserves the compatible legacy credentials.
// SS-2022 keys are generated independently once, never regenerated on resume.
func (s *Service) ImportIdentity(ctx context.Context, user model.User, legacyID int64, uuidValue, trojan, chacha string, disabled ...bool) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if legacyID <= 0 || legacyID >= 1<<40 || user.ID == "" || trojan == "" || chacha == "" {
		return Identity{}, ErrInvalid
	}
	parsed, err := uuid.Parse(uuidValue)
	if err != nil || parsed == uuid.Nil {
		return Identity{}, ErrInvalid
	}
	current, err := s.repo.XNodeIdentityByUser(ctx, user.ID)
	if err == nil {
		credentials, err := loadCredentials(s.vault, current)
		if err != nil {
			return current, err
		}
		if credentials.UUID != uuidValue || credentials.Trojan != trojan || credentials.Shadowsocks != chacha || current.LegacyUserID != strconv.FormatInt(legacyID, 10) {
			return current, ErrConflict
		}
		return current, nil
	}
	if !isMissing(err) {
		return Identity{}, err
	}
	credentials, err := NewCredentials()
	if err != nil {
		return Identity{}, err
	}
	credentials.UUID, credentials.Trojan, credentials.Shadowsocks = uuidValue, trojan, chacha
	identity := Identity{ProxyUserID: legacyID, UserID: user.ID, LegacyUserID: strconv.FormatInt(legacyID, 10), CreatedAt: user.CreatedAt}
	if len(disabled) > 0 {
		identity.AccessDisabled = disabled[0]
	}
	identity.CredentialsCipher, err = sealCredentials(s.vault, identity, credentials)
	if err != nil {
		return identity, err
	}
	if err = s.newSubscription(&identity); err != nil {
		return identity, err
	}
	return s.repo.SaveXNodeIdentity(ctx, identity)
}
