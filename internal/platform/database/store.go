package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

var (
	ErrAccountUnavailable = errors.New("account is unavailable")
	ErrNotFound           = errors.New("record not found")
	// ErrConflict indicates that a uniqueness or state transition invariant was violated.
	ErrConflict = errors.New("record conflicts with current state")
	// ErrInsufficientBalance indicates that a debit would make an account negative.
	ErrInsufficientBalance = errors.New("insufficient TXB balance")
	// ErrStockUnavailable indicates that a limited squad has no remaining user reservation.
	ErrStockUnavailable = errors.New("squad stock is unavailable")
)

// Store implements persistent repositories on PostgreSQL.
type Store struct {
	cleanupAdmins        atomic.Pointer[accountCleanupAdminSet]
	periodScanMu         sync.Mutex
	periodScanCursor     string
	firewallPages        firewallPageCache
	pricingExposureCache *pricing.AnalysisCache
	pricingCache         *pricing.AnalysisCache
	pricingExportSlot    chan struct{}
	pricingComputeSlot   chan struct{}
	pricingHealth        *pricing.HealthRegistry
	diyCleanupCursor     string
	firewallDetectionMu  sync.Mutex
	firewallWindows      map[string]firewallDetectionWindow
	firewallSweepMu      sync.Mutex
	firewallSweepCursor  string // protected by firewallSweepMu; bounded operational pruning
	firewallRetentionAt  time.Time
	pmContentCipher      pmContentCipher
	db                   *sql.DB
	logger               *slog.Logger
	writeMu              sync.Mutex
	randomMu             sync.Mutex
	groupFactsMu         sync.Mutex
	groupFacts           map[groupMessageFactKey]groupMessageFact
}

// NewStore creates an application store.
func NewStore(db *sql.DB) *Store {
	return &Store{pricingExposureCache: pricing.NewExposureCache(), pricingCache: pricing.NewAnalysisCache(), db: db, groupFacts: make(map[groupMessageFactKey]groupMessageFact), pricingHealth: pricing.NewHealthRegistry(), pricingExportSlot: make(chan struct{}, 1), pricingComputeSlot: make(chan struct{}, 1)}
}

// UpsertTelegramUser creates or refreshes a user from validated Telegram data.
func (s *Store) UpsertTelegramUser(ctx context.Context, profile model.TelegramProfile, isAdmin bool) (model.User, bool, error) {
	return s.upsertAuthenticatedTelegram(ctx, profile, isAdmin)
}

// UserByID returns a user by internal ID.
func (s *Store) UserByID(ctx context.Context, userID string) (model.User, error) {
	condition := `old_user_id ~ '[^0-9]'`

	return scanUser(s.db.QueryRowContext(ctx, userSelect+` WHERE users.id=COALESCE((SELECT user_id FROM member_identity_aliases WHERE old_user_id=$1 AND `+condition+`),$2)`, userID, userID))
}

// UserByTelegramID returns a user by trusted Telegram ID.
func (s *Store) UserByTelegramID(ctx context.Context, telegramID int64) (model.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+` WHERE users.telegram_id=$1`, telegramID))
}

// Assigned usernames were removed with the Telegram identity migration. Member
// account references come from AccountName, so the legacy field stays empty.
const userColumns = `users.id,users.telegram_id,users.telegram_first_name,users.telegram_last_name,users.telegram_username,
	users.role,users.onboarding_state,users.group_joined,users.channel_joined,users.policy_accepted_at,
	users.accepted_agreement_revision,users.proxy_user_id,users.recovery_reason,users.new_user,users.inviter_id,
	users.notification_locale,users.display_currency,users.created_at,users.updated_at,
	users.account_status,users.account_generation,users.bot_blocked`

const userSelect = `SELECT ` + userColumns + ` FROM users`

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(row rowScanner) (model.User, error) {
	return scanUserWith(row)
}

func scanUserWith(row rowScanner, extra ...any) (model.User, error) {
	var user model.User
	var policy, recoveryReason sql.NullString
	var proxyID sql.NullString
	var groupJoined, channelJoined, newUser int
	var inviterID sql.NullInt64
	var createdAt, updatedAt string
	destinations := []any{&user.ID, &user.TelegramID, &user.TelegramFirstName, &user.TelegramLastName, &user.TelegramUsername,
		&user.Role, &user.OnboardingState, &groupJoined, &channelJoined, &policy, &user.AgreementRevision, &proxyID, &recoveryReason,
		&newUser, &inviterID, &user.NotificationLocale, &user.DisplayCurrency, &createdAt, &updatedAt,
		&user.AccountStatus, &user.AccountGeneration, &user.BotBlocked}
	destinations = append(destinations, extra...)
	if err := row.Scan(destinations...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.User{}, ErrNotFound
		}
		return model.User{}, fmt.Errorf("scan user: %w", err)
	}
	user.GroupJoined = groupJoined == 1
	user.ChannelJoined = channelJoined == 1
	user.NewUser = newUser == 1
	if inviterID.Valid {
		user.InviterID = &inviterID.Int64
	}
	if policy.Valid {
		parsed, err := parseStamp(policy.String)
		if err != nil {
			return model.User{}, fmt.Errorf("parse policy timestamp: %w", err)
		}
		user.PolicyAcceptedAt = &parsed
	}
	user.ProxyUserID = nullableString(proxyID)
	user.RecoveryReason = recoveryReason.String
	var err error
	user.CreatedAt, err = parseStamp(createdAt)
	if err != nil {
		return model.User{}, fmt.Errorf("parse user creation timestamp: %w", err)
	}
	user.UpdatedAt, err = parseStamp(updatedAt)
	if err != nil {
		return model.User{}, fmt.Errorf("parse user update timestamp: %w", err)
	}
	return user, nil
}

// ReplaceSession stores one current opaque session per user.
func (s *Store) retryAttempt_ReplaceSession(ctx context.Context, tokenHash []byte, userID string, expiresAt time.Time) error {

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin session replacement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT account_status='active' AND bot_blocked=0 FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&active); err != nil {
		return err
	}
	if !active {
		return ErrAccountUnavailable
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=$1 AND scope='miniapp'`, userID); err != nil {
		return fmt.Errorf("remove previous session: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at,created_at,renewed_at) VALUES($1,$2,$3,$4,$5)`, tokenHash, userID, stamp(expiresAt), stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit session replacement: %w", err)
	}
	return nil
}

// UserBySession returns the owner of a non-expired session.
func (s *Store) UserBySession(ctx context.Context, tokenHash []byte, now time.Time) (model.User, error) {
	user, _, err := s.authenticatedSession(ctx, tokenHash, "miniapp", now)
	return user, err
}

// UpdateMembership persists canonical Telegram membership results without
// changing onboarding. Community access is independent from account setup.

func (s *Store) UpdateMembership(ctx context.Context, userID string, groupJoined, channelJoined bool) (model.User, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx, `UPDATE users SET group_joined=$1,channel_joined=$2,updated_at=$3 WHERE id=$4`,
		boolInt(groupJoined), boolInt(channelJoined), stamp(time.Now().UTC()), userID)
	if err != nil {
		return model.User{}, fmt.Errorf("update membership: %w", err)
	}
	return s.UserByID(ctx, userID)
}
