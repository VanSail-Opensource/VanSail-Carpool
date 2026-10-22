package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// MigrationPanelAccounts returns linked panel accounts only, including records
// beyond the administrative page limit. Credential import checks both identities.
func (s *Store) MigrationPanelAccounts(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, userSelect+` WHERE users.remna_user_id IS NOT NULL ORDER BY users.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := []model.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// XNodeMigrationAccountRequired keeps missing source accounts blocking whenever
// a purchase or an already imported native identity needs their accounting.
// It changes neither the legacy link nor any financial record.
func (s *Store) XNodeMigrationAccountRequired(ctx context.Context, userID string) (bool, error) {
	var required bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=?) OR EXISTS(SELECT 1 FROM xnode_identities WHERE user_id=?)`, userID, userID).Scan(&required)
	return required, err
}
