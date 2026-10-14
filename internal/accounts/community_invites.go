package accounts

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// CreateCommunityInvite creates one short-lived, identity-bound join-request
// link after checking canonical membership and strict active-combo eligibility.
func (s *Service) CreateCommunityInvite(ctx context.Context, user model.User, space CommunitySpace) (string, time.Time, error) {
	if !space.valid() {
		return "", time.Time{}, errors.New("unsupported community space")
	}
	refreshed, joined, err := s.checkCommunitySpaceMembership(ctx, user, space)
	if err != nil {
		return "", time.Time{}, err
	}
	if joined {
		return "", time.Time{}, ErrCommunityAlreadyJoined
	}
	active, err := s.hasActiveCombo(ctx, refreshed.ID)
	if err != nil {
		return "", time.Time{}, err
	}
	if !active {
		return "", time.Time{}, ErrActiveComboRequired
	}
	chatID, chatIDValue, err := s.communityChatID(ctx, space)
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt := s.now().UTC().Add(30 * time.Minute)
	inviteName, err := s.signedInviteName(ctx, refreshed.TelegramID, chatID, expiresAt)
	if err != nil {
		return "", time.Time{}, err
	}
	link, err := s.telegram.CreateJoinRequestInvite(ctx, chatIDValue, inviteName, expiresAt)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("create %s invite: %w", space, err)
	}
	return link, expiresAt, nil
}

// HandleSignedJoinRequest verifies identity, configured chat, and expiry before
// rechecking entitlement immediately before approval. A lapsed entitlement is
// actively declined and its single-use invite is revoked.
func (s *Service) HandleSignedJoinRequest(ctx context.Context, telegramID, chatID int64, inviteLink, inviteName string, expiresAt time.Time) error {
	if strings.TrimSpace(inviteLink) == "" || len(inviteName) > 32 || !s.now().UTC().Before(expiresAt.UTC()) {
		return ErrInvalidAuthentication
	}
	chatIDValue, err := s.configuredCommunityChat(ctx, chatID)
	if err != nil {
		return err
	}
	parts := strings.Split(inviteName, ".")
	if len(parts) != 2 {
		return ErrInvalidAuthentication
	}
	signedTelegramID, err := strconv.ParseInt(parts[0], 36, 64)
	if err != nil || signedTelegramID != telegramID {
		return ErrInvalidAuthentication
	}
	expected, err := s.inviteSignature(ctx, signedTelegramID, chatID, expiresAt)
	if err != nil || !hmac.Equal([]byte(parts[1]), []byte(expected)) {
		return ErrInvalidAuthentication
	}
	user, err := s.repository.UserByTelegramID(ctx, telegramID)
	if err != nil {
		return err
	}
	active, err := s.hasActiveCombo(ctx, user.ID)
	if err != nil {
		return err
	}
	if !active {
		if err := s.declineAndRevoke(ctx, chatIDValue, telegramID, inviteLink); err != nil {
			return err
		}
		return ErrActiveComboRequired
	}
	if err := s.telegram.ApproveJoinRequest(ctx, chatIDValue, telegramID); err != nil {
		present, membershipErr := s.telegram.GetMembership(ctx, chatIDValue, telegramID)
		if membershipErr != nil || !present {
			return err
		}
	}
	return s.telegram.RevokeInviteLink(ctx, chatIDValue, inviteLink)
}

func (s *Service) communityChatID(ctx context.Context, space CommunitySpace) (int64, string, error) {
	if !space.valid() {
		return 0, "", errors.New("unsupported community space")
	}
	chatIDValue, err := s.settings.Plaintext(ctx, "telegram."+string(space)+"_chat_id")
	if err != nil {
		return 0, "", fmt.Errorf("load %s chat: %w", space, err)
	}
	chatID, err := strconv.ParseInt(chatIDValue, 10, 64)
	if err != nil || chatID == 0 {
		return 0, "", fmt.Errorf("invalid %s chat id", space)
	}
	return chatID, chatIDValue, nil
}

func (s *Service) configuredCommunityChat(ctx context.Context, chatID int64) (string, error) {
	for _, space := range []CommunitySpace{CommunityGroup, CommunityChannel} {
		configuredID, chatIDValue, err := s.communityChatID(ctx, space)
		if err != nil {
			return "", err
		}
		if configuredID == chatID {
			return chatIDValue, nil
		}
	}
	return "", ErrInvalidAuthentication
}

func (s *Service) declineAndRevoke(ctx context.Context, chatID string, telegramID int64, inviteLink string) error {
	declineErr := s.telegram.DeclineJoinRequest(ctx, chatID, telegramID)
	revokeErr := s.telegram.RevokeInviteLink(ctx, chatID, inviteLink)
	if declineErr != nil {
		return fmt.Errorf("decline Telegram join request: %w", declineErr)
	}
	if revokeErr != nil {
		return fmt.Errorf("revoke Telegram invite: %w", revokeErr)
	}
	return nil
}

func (s *Service) signedInviteName(ctx context.Context, telegramID, chatID int64, expiresAt time.Time) (string, error) {
	signature, err := s.inviteSignature(ctx, telegramID, chatID, expiresAt)
	if err != nil {
		return "", err
	}
	name := strconv.FormatInt(telegramID, 36) + "." + signature
	if len(name) > 32 {
		return "", errors.New("Telegram invite identity exceeds name limit")
	}
	return name, nil
}

func (s *Service) inviteSignature(ctx context.Context, telegramID, chatID int64, expiresAt time.Time) (string, error) {
	secret, err := s.settings.Plaintext(ctx, "telegram.webhook_secret")
	if err != nil || strings.TrimSpace(secret) == "" {
		return "", errors.New("Telegram invite signing secret is unavailable")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d|%d|%d", telegramID, chatID, expiresAt.UTC().Unix())
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:12]), nil
}
