package xnode

import (
	"context"
	"crypto/sha256"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"strconv"
)

func (s *Service) Identity(ctx context.Context, id string) (Identity, error) {
	number, err := strconv.ParseInt(id, 10, 64)
	if err != nil || number <= 0 || number >= 1<<53 {
		return Identity{}, ErrInvalid
	}
	return s.repo.XNodeIdentity(ctx, number)
}
func (s *Service) IdentityForUser(ctx context.Context, id string) (Identity, error) {
	return s.repo.XNodeIdentityByUser(ctx, id)
}
func (s *Service) IdentityUser(ctx context.Context, identity Identity) (model.User, error) {
	if identity.MemberOwnerID() == "" {
		return model.User{}, ErrNotFound
	}
	return s.repo.UserByID(ctx, identity.MemberOwnerID())
}
func (s *Service) UserByUsername(ctx context.Context, name string) (model.User, error) {
	return s.repo.UserByUsername(ctx, name)
}
func (s *Service) UserByTelegramID(ctx context.Context, id int64) (model.User, error) {
	return s.repo.UserByTelegramID(ctx, id)
}

// EnsureIdentity allocates credentials only for an explicitly provisioned panel account.
// The presence of an identity never grants admission; paid access is checked on compilation.
func (s *Service) EnsureIdentity(ctx context.Context, user model.User) (Identity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, err := s.repo.XNodeIdentityByUser(ctx, user.ID)
	if err == nil {
		return existing, nil
	}
	if !isMissing(err) {
		return Identity{}, err
	}
	credentials, err := NewCredentials()
	if err != nil {
		return Identity{}, err
	}
	identity := Identity{UserID: user.ID}
	identity.CredentialsCipher, err = sealCredentials(s.vault, identity, credentials)
	if err != nil {
		return identity, err
	}
	if err = s.newSubscription(&identity); err != nil {
		return identity, err
	}
	return s.repo.SaveXNodeIdentity(ctx, identity)
}
func (s *Service) newSubscription(identity *Identity) error {
	token, err := ids.Token(32)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(token))
	identity.SubscriptionHash = hash[:]
	identity.SubscriptionCipher, err = s.vault.Encrypt("xnode.subscription."+identityScope(*identity), token)
	return err
}
func (s *Service) SubscriptionURL(identity Identity) (string, error) {
	token, err := s.vault.Decrypt("xnode.subscription."+identityScope(identity), identity.SubscriptionCipher)
	if err != nil {
		return "", err
	}
	u := *s.publicURL
	u.Path = "/subscription/" + token
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
func (s *Service) RevokeSubscription(ctx context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity, err := s.Identity(ctx, id)
	if err != nil {
		return "", err
	}
	if isPAYGIdentity(identity) {
		return "", ErrInvalid
	}
	if err = s.newSubscription(&identity); err != nil {
		return "", err
	}
	if _, err = s.repo.SaveXNodeIdentity(ctx, identity); err != nil {
		return "", err
	}
	return s.SubscriptionURL(identity)
}
