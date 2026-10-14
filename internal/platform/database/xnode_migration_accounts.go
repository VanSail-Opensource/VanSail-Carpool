package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// MigrationPanelAccounts returns linked panel accounts only, including records
// beyond the administrative page limit. Credential import checks both identities.
func (s *Store) MigrationPanelAccounts(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, userSelect+` WHERE users.remna_user_id IS NOT NULL AND users.username IS NOT NULL ORDER BY users.id`)
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
