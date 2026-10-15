package apitoken

import (
	"context"
	"crypto/sha256"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

type Service struct {
	repo   Repository
	admins map[int64]bool
	now    func() time.Time
}

func New(repo Repository, admins []int64) *Service {
	allowed := map[int64]bool{}
	for _, id := range admins {
		allowed[id] = true
	}
	return &Service{repo: repo, admins: allowed, now: time.Now}
}

func (s *Service) isAdmin(user model.User) bool {
	return user.Role == "admin" && s.admins[user.TelegramID]
}

func (s *Service) Create(ctx context.Context, user model.User, input CreateInput) (Issued, error) {
	if !s.isAdmin(user) {
		return Issued{}, ErrUnauthorized
	}
	name := strings.TrimSpace(input.Name)
	scopes, err := normalizeScopes(input.Scopes)
	if err != nil || name == "" || utf8.RuneCountInString(name) > 80 || strings.ContainsFunc(name, unicode.IsControl) {
		return Issued{}, ErrInvalid
	}
	days := 30
	if input.ExpiresInDays != nil {
		days = *input.ExpiresInDays
	}
	if days < 0 || days > 365 {
		return Issued{}, ErrInvalid
	}
	random, err := ids.Token(32)
	if err != nil {
		return Issued{}, err
	}
	id, err := ids.New()
	if err != nil {
		return Issued{}, err
	}
	raw := "vsc_" + random
	digest := sha256.Sum256([]byte(raw))
	item := Token{ID: id, UserID: user.ID, Name: name, Prefix: raw[:12], Hash: digest[:], Scopes: scopes, CreatedAt: s.now().UTC()}
	if days > 0 {
		at := item.CreatedAt.Add(time.Duration(days) * 24 * time.Hour)
		item.ExpiresAt = &at
	}
	if err = s.repo.CreateAdminAPIToken(ctx, item); err != nil {
		return Issued{}, err
	}
	return Issued{Item: item, Token: raw}, nil
}

func (s *Service) List(ctx context.Context, user model.User) ([]Token, error) {
	if !s.isAdmin(user) {
		return nil, ErrUnauthorized
	}
	return s.repo.AdminAPITokens(ctx, user.ID)
}

func (s *Service) Revoke(ctx context.Context, user model.User, id string) error {
	if !s.isAdmin(user) {
		return ErrUnauthorized
	}
	return s.repo.RevokeAdminAPIToken(ctx, id, user.ID, s.now().UTC())
}

func (s *Service) Authenticate(ctx context.Context, raw string) (Principal, error) {
	if len(raw) != 47 || !strings.HasPrefix(raw, "vsc_") {
		return Principal{}, ErrUnauthorized
	}
	for _, ch := range raw[4:] {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_' || ch == '-') {
			return Principal{}, ErrUnauthorized
		}
	}
	digest := sha256.Sum256([]byte(raw))
	item, err := s.repo.AdminAPITokenByHash(ctx, digest[:])
	if err != nil {
		return Principal{}, err
	}
	now := s.now().UTC()
	if item.RevokedAt != nil || item.ExpiresAt != nil && !now.Before(*item.ExpiresAt) {
		return Principal{}, ErrUnauthorized
	}
	if _, err = normalizeScopes(item.Scopes); err != nil {
		return Principal{}, ErrUnauthorized
	}
	user, err := s.repo.UserByID(ctx, item.UserID)
	if err != nil {
		return Principal{}, err
	}
	if !s.isAdmin(user) {
		return Principal{}, ErrUnauthorized
	}
	if item.LastUsedAt == nil || now.Sub(*item.LastUsedAt) >= 5*time.Minute {
		if err = s.repo.TouchAdminAPIToken(ctx, item.ID, now); err != nil {
			return Principal{}, err
		}
	}
	return Principal{Item: item, User: user}, nil
}
