package database

import "context"

// RegisterPanelEntry creates a first-entry row only for never-onboarded accounts.
// Entering while CAPTCHA is off qualifies the account without a challenge.
func (s *Store) retryAttempt_RegisterPanelEntry(ctx context.Context, telegramID int64, challenge bool) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO panel_entry_verification(telegram_id,checked)
		SELECT telegram_id,0 FROM users WHERE telegram_id=? AND policy_accepted_at IS NULL AND accepted_agreement_revision=0
		ON CONFLICT(telegram_id) DO NOTHING`, telegramID); err != nil {
		return err
	}
	if !challenge {
		if _, err := tx.ExecContext(ctx, `UPDATE panel_entry_verification SET checked=1 WHERE telegram_id=?`, telegramID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PanelEntryQualified uses retained agreement evidence after the temporary row is removed.
func (s *Store) PanelEntryQualified(ctx context.Context, telegramID int64) (bool, error) {
	var qualified bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE telegram_id=?
		AND (policy_accepted_at IS NOT NULL OR accepted_agreement_revision>0 OR EXISTS(SELECT 1 FROM panel_entry_verification v WHERE v.telegram_id=users.telegram_id AND v.checked=1)))`, telegramID).Scan(&qualified)
	return qualified, err
}

// VerifyPanelEntry updates an existing authenticated entry; it cannot recreate an onboarded row.
func (s *Store) VerifyPanelEntry(ctx context.Context, telegramID int64) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `UPDATE panel_entry_verification SET checked=1 WHERE telegram_id=?
		AND EXISTS(SELECT 1 FROM users WHERE telegram_id=? AND policy_accepted_at IS NULL AND accepted_agreement_revision=0)`, telegramID, telegramID)
	return err
}
