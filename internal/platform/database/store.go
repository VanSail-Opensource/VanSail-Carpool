package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var (
	ErrNotFound = errors.New("record not found")
	// ErrConflict indicates that a uniqueness or state transition invariant was violated.
	ErrConflict = errors.New("record conflicts with current state")
	// ErrInsufficientBalance indicates that a debit would make an account negative.
	ErrInsufficientBalance = errors.New("insufficient TXB balance")
	// ErrStockUnavailable indicates that a limited squad has no remaining user reservation.
	ErrStockUnavailable = errors.New("squad stock is unavailable")
)

// Store implements persistent repositories on top of SQLite.
type Store struct {
	diyCleanupCursor    string
	firewallDetectionMu sync.Mutex
	firewallWindows     map[string]firewallDetectionWindow
	firewallSweepCursor string // protected by writeMu; bounded operational pruning
	pmContentCipher     pmContentCipher
	db                  *sql.DB
	logger              *slog.Logger
	writeMu             sync.Mutex
	randomMu            sync.Mutex
	groupFactsMu        sync.Mutex
	groupFacts          map[groupMessageFactKey]groupMessageFact
}

// NewStore creates an application store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, groupFacts: make(map[groupMessageFactKey]groupMessageFact)}
}

// UpsertTelegramUser creates or refreshes a user from validated Telegram data.
func (s *Store) UpsertTelegramUser(ctx context.Context, profile model.TelegramProfile, isAdmin bool) (model.User, bool, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	now := time.Now().UTC()
	role := "user"
	if isAdmin {
		role = "admin"
	}
	if profile.ID <= 0 || profile.ID >= 1<<52 {
		return model.User{}, false, ErrConflict
	}
	userID := strconv.FormatInt(profile.ID, 10)
	var existed bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE telegram_id=?)`, profile.ID).Scan(&existed); err != nil {
		return model.User{}, false, err
	}
	locale := affiliates.NormalizeLocale(profile.LanguageCode)
	_, err := s.db.ExecContext(ctx, `INSERT INTO users(id,telegram_id,telegram_first_name,telegram_last_name,telegram_username,role,new_user,notification_locale,auto_traffic_reset_enabled,created_at,updated_at)
		VALUES(?,?,?,?,?,?,0,?,1,?,?) ON CONFLICT(telegram_id) DO UPDATE SET telegram_first_name=excluded.telegram_first_name,
		telegram_last_name=excluded.telegram_last_name,telegram_username=excluded.telegram_username,
		role=excluded.role,new_user=0,notification_locale=excluded.notification_locale,updated_at=excluded.updated_at`,
		userID, profile.ID, profile.FirstName, profile.LastName, profile.Username, role, locale, stamp(now), stamp(now))
	if err != nil {
		return model.User{}, false, fmt.Errorf("upsert Telegram user: %w", err)
	}
	user, err := s.UserByTelegramID(ctx, profile.ID)
	if err != nil {
		return model.User{}, false, err
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO balances(user_id,txb_minor,updated_at) VALUES(?,?,?) ON CONFLICT(user_id) DO NOTHING`, user.ID, 0, stamp(now)); err != nil {
		return model.User{}, false, fmt.Errorf("initialize balance: %w", err)
	}
	return user, !existed, nil
}

// UserByID returns a user by internal ID.
func (s *Store) UserByID(ctx context.Context, userID string) (model.User, error) {
	condition := `old_user_id GLOB '*[^0-9]*'`
	if IsPostgres(s.db) {
		condition = `old_user_id ~ '[^0-9]'`
	}
	return scanUser(s.db.QueryRowContext(ctx, userSelect+` WHERE users.id=COALESCE((SELECT user_id FROM member_identity_aliases WHERE old_user_id=? AND `+condition+`),?)`, userID, userID))
}

// UserByTelegramID returns a user by trusted Telegram ID.
func (s *Store) UserByTelegramID(ctx context.Context, telegramID int64) (model.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+` WHERE users.telegram_id=?`, telegramID))
}

// Assigned usernames were removed with the Telegram identity migration. Member
// account references come from AccountName, so the legacy field stays empty.
const userColumns = `users.id,users.telegram_id,users.telegram_first_name,users.telegram_last_name,users.telegram_username,
	users.role,users.onboarding_state,users.group_joined,users.channel_joined,users.policy_accepted_at,
	users.accepted_agreement_revision,users.proxy_user_id,users.recovery_reason,users.new_user,users.inviter_id,
	users.notification_locale,users.display_currency,users.auto_traffic_reset_enabled,users.created_at,users.updated_at`

const userSelect = `SELECT ` + userColumns + ` FROM users`

type rowScanner interface{ Scan(dest ...any) error }

func scanUser(row rowScanner) (model.User, error) {
	return scanUserWith(row)
}

func scanUserWith(row rowScanner, extra ...any) (model.User, error) {
	var user model.User
	var policy, recoveryReason sql.NullString
	var proxyID sql.NullString
	var groupJoined, channelJoined, newUser, autoTrafficReset int
	var inviterID sql.NullInt64
	var createdAt, updatedAt string
	destinations := []any{&user.ID, &user.TelegramID, &user.TelegramFirstName, &user.TelegramLastName, &user.TelegramUsername,
		&user.Role, &user.OnboardingState, &groupJoined, &channelJoined, &policy, &user.AgreementRevision, &proxyID, &recoveryReason,
		&newUser, &inviterID, &user.NotificationLocale, &user.DisplayCurrency, &autoTrafficReset, &createdAt, &updatedAt}
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
	user.AutoTrafficResetEnabled = autoTrafficReset == 1
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
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin session replacement: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id=? AND scope='miniapp'`, userID); err != nil {
		return fmt.Errorf("remove previous session: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions(token_hash,user_id,expires_at,created_at,renewed_at) VALUES(?,?,?,?,?)`, tokenHash, userID, stamp(expiresAt), stamp(now), stamp(now)); err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit session replacement: %w", err)
	}
	return nil
}

// UserBySession returns the owner of a non-expired session.
func (s *Store) UserBySession(ctx context.Context, tokenHash []byte, now time.Time) (model.User, error) {
	return scanUser(s.db.QueryRowContext(ctx, userSelect+` JOIN sessions ON sessions.user_id=users.id WHERE sessions.token_hash=? AND sessions.scope='miniapp' AND sessions.expires_at>?`, tokenHash, stamp(now)))
}

// DeleteExpiredSessions removes stale session rows.

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at<=?`, stamp(now)); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	return nil
}

// UpdateMembership persists canonical Telegram membership results without
// changing onboarding. Community access is independent from account setup.

func (s *Store) UpdateMembership(ctx context.Context, userID string, groupJoined, channelJoined bool) (model.User, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	_, err := s.db.ExecContext(ctx, `UPDATE users SET group_joined=?,channel_joined=?,updated_at=? WHERE id=?`,
		boolInt(groupJoined), boolInt(channelJoined), stamp(time.Now().UTC()), userID)
	if err != nil {
		return model.User{}, fmt.Errorf("update membership: %w", err)
	}
	return s.UserByID(ctx, userID)
}
