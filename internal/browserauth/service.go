// Package browserauth owns isolated browser-admin identities and one-use handoffs.
package browserauth

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// SessionTTL is the rolling inactivity window for independent browser devices.
const SessionTTL = 30 * 24 * time.Hour

// ErrUnauthorized denotes an expired session or an identity outside the administrator allowlist.
var ErrUnauthorized = errors.New("browser administrator authentication required")

// Service uses existing account records and independent browser device sessions.
type Service struct {
	store     *database.Store
	publicURL *url.URL
	admins    map[int64]struct{}
	now       func() time.Time
}

// New constructs the browser scope without introducing a second queue or account store.
func New(store *database.Store, publicURL *url.URL, admins []int64) *Service {
	allowed := make(map[int64]struct{}, len(admins))
	for _, id := range admins {
		allowed[id] = struct{}{}
	}
	return &Service{store: store, publicURL: publicURL, admins: allowed, now: time.Now}
}

// IsAdmin checks the configured identity allowlist and the current persisted role.
func (s *Service) IsAdmin(user model.User) bool {
	_, ok := s.admins[user.TelegramID]
	return ok && user.Role == "admin"
}

// Session resolves only a browser-scoped session and rechecks admin eligibility.
func (s *Service) Session(ctx context.Context, token string) (model.User, time.Time, error) {
	if len(token) != 43 {
		return model.User{}, time.Time{}, ErrUnauthorized
	}
	hash := sha256.Sum256([]byte(token))
	user, expires, err := s.store.BrowserSession(ctx, hash[:], s.now().UTC())
	if errors.Is(err, database.ErrNotFound) {
		return model.User{}, time.Time{}, ErrUnauthorized
	}
	if err != nil {
		return model.User{}, time.Time{}, fmt.Errorf("resolve browser session: %w", err)
	}
	if !s.IsAdmin(user) {
		return model.User{}, time.Time{}, ErrUnauthorized
	}
	return user, expires, nil
}

// Renew extends a live browser session at most once daily.
func (s *Service) Renew(ctx context.Context, token string) (model.User, time.Time, bool, error) {
	_, _, err := s.Session(ctx, token)
	if err != nil {
		return model.User{}, time.Time{}, false, err
	}
	now := s.now().UTC()
	hash := sha256.Sum256([]byte(token))
	renewed, err := s.store.RenewBrowserSession(ctx, hash[:], now, now.Add(SessionTTL))
	if err != nil {
		return model.User{}, time.Time{}, false, err
	}
	// Concurrent tabs can win the daily update. Read its actual expiry before
	// returning cookies, and reject a device revoked while renewal was in flight.
	user, expires, err := s.Session(ctx, token)
	return user, expires, renewed, err
}

// Logout revokes only the current browser session.
func (s *Service) Logout(ctx context.Context, token string) error {
	hash := sha256.Sum256([]byte(token))
	return s.store.DeleteBrowserSession(ctx, hash[:])
}

// ValidDestination confines handoff redirects to the browser-admin router.
func ValidDestination(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && !parsed.IsAbs() && parsed.Host == "" && parsed.User == nil && parsed.Fragment == "" && (parsed.Path == "/admin" || strings.HasPrefix(parsed.Path, "/admin/")) && !strings.Contains(parsed.Path, "\\") && !strings.Contains(parsed.Path, "..") && !strings.ContainsAny(raw, "\r\n")
}

// Link creates a five-minute bearer handoff whose plaintext is never stored.
func (s *Service) Link(ctx context.Context, user model.User, returnTo string) (string, time.Time, error) {
	if !s.IsAdmin(user) {
		return "", time.Time{}, ErrUnauthorized
	}
	if returnTo == "" {
		returnTo = "/admin/settings"
	}
	if !ValidDestination(returnTo) {
		return "", time.Time{}, errors.New("invalid administrator destination")
	}
	token, err := ids.Token(32)
	if err != nil {
		return "", time.Time{}, err
	}
	expires := s.now().UTC().Add(5 * time.Minute)
	hash := sha256.Sum256([]byte(token))
	if err = s.store.CreateBrowserAccessLink(ctx, hash[:], user.ID, returnTo, expires); err != nil {
		return "", time.Time{}, err
	}
	link := s.publicURL.ResolveReference(&url.URL{Path: "/admin/access", Fragment: "token=" + token})
	return link.String(), expires, nil
}

// Exchange atomically consumes the handoff and creates its independent browser device.
func (s *Service) Exchange(ctx context.Context, linkToken string) (model.User, string, time.Time, string, error) {
	if len(linkToken) != 43 {
		return model.User{}, "", time.Time{}, "", ErrUnauthorized
	}
	token, err := ids.Token(32)
	if err != nil {
		return model.User{}, "", time.Time{}, "", err
	}
	now := s.now().UTC()
	expires := now.Add(SessionTTL)
	linkHash := sha256.Sum256([]byte(linkToken))
	hash := sha256.Sum256([]byte(token))
	id, returnTo, err := s.store.ExchangeBrowserAccessLink(ctx, linkHash[:], hash[:], now, expires)
	if err != nil {
		return model.User{}, "", time.Time{}, "", ErrUnauthorized
	}
	user, err := s.store.UserByID(ctx, id)
	if err != nil || !s.IsAdmin(user) || !ValidDestination(returnTo) {
		revokeErr := s.store.DeleteBrowserSession(ctx, hash[:])
		return model.User{}, "", time.Time{}, "", errors.Join(ErrUnauthorized, revokeErr)
	}
	return user, token, expires, returnTo, nil
}
