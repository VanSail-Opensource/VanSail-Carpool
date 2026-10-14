package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// SetDisplayCurrency saves one member-facing currency preference.
func (s *Store) SetDisplayCurrency(ctx context.Context, userID, currency string) (model.User, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	result, err := s.db.ExecContext(ctx, `UPDATE users SET display_currency=?,updated_at=? WHERE id=?`, currency, stamp(time.Now().UTC()), userID)
	if err != nil {
		return model.User{}, fmt.Errorf("save display currency: %w", err)
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return model.User{}, rowsErr
		}
		return model.User{}, ErrNotFound
	}
	return s.UserByID(ctx, userID)
}
