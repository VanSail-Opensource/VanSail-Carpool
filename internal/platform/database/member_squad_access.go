package database

import (
	"context"
	"database/sql"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

// EnabledSquadsForUser applies native ownership without requiring a legacy link.
func (s *Store) EnabledSquadsForUser(ctx context.Context, userID string, owned []string) ([]string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	enabled, err := enabledSquadsTx(ctx, tx, userID, owned)
	if err != nil {
		return nil, err
	}
	return enabled, tx.Commit()
}

func enabledSquadsTx(ctx context.Context, tx *sql.Tx, userID string, owned []string) ([]string, error) {
	disabled, err := disabledSquadsFrom(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	enabled := purchaseops.EnabledSquads(uniqueSorted(owned), disabled)
	if len(enabled) == 1 && slices.Contains(disabled, enabled[0]) {
		if _, err := tx.ExecContext(ctx, `DELETE FROM user_disabled_squads WHERE user_id=? AND squad_uuid=?`, userID, enabled[0]); err != nil {
			return nil, err
		}
	}
	return enabled, nil
}
