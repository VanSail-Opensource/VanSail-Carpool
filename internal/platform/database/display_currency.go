package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// SetDisplayCurrency saves one member-facing currency preference.
func (s *Store) SetDisplayCurrency(ctx context.Context, userID, currency string) (model.User, error) {

	user, err := scanUser(s.db.QueryRowContext(ctx, `UPDATE users SET display_currency=$1,updated_at=$2 WHERE id=$3 RETURNING `+userColumns, currency, stamp(time.Now().UTC()), userID))
	if err != nil {
		return model.User{}, fmt.Errorf("save display currency: %w", err)
	}
	return user, nil
}
