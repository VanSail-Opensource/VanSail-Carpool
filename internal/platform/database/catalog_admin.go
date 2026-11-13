package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// SaveAdminCombo commits the catalog, dependent resynchronization and audit
// together and returns the exact snapshot written by that transaction.
func (s *Store) retryAttempt_SaveAdminCombo(ctx context.Context, actorID string, input ComboInput) (model.Combo, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.Combo{}, err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return model.Combo{}, err
	}
	now := time.Now().UTC()
	combo, err := saveComboTx(ctx, tx, input, now)
	if err != nil {
		return model.Combo{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "combo.save", "combo", combo.ID, map[string]string{"name": combo.Name}, now); err != nil {
		return model.Combo{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.Combo{}, err
	}
	return combo, nil
}

// DeleteAdminCombo hides the offer without modifying purchases that refer to it.
func (s *Store) retryAttempt_DeleteAdminCombo(ctx context.Context, actorID, comboID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return err
	}
	now := time.Now().UTC()
	result, err := tx.ExecContext(ctx, `UPDATE combos SET active=0,updated_at=$1 WHERE id=$2`, stamp(now), comboID)
	if err != nil {
		return err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return rowsErr
	} else if affected != 1 {
		return ErrNotFound
	}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "combo.hide", "combo", comboID, nil, now); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveAdminSquadProduct pairs every sparse override write or removal with its
// administrator audit, retaining omitted activation and feature material.
func (s *Store) retryAttempt_SaveAdminSquadProduct(ctx context.Context, actorID string, input SquadProductInput) (model.SquadProduct, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return model.SquadProduct{}, err
	}
	defer tx.Rollback()
	if err = catalogAdminActorTx(ctx, tx, actorID); err != nil {
		return model.SquadProduct{}, err
	}
	now := time.Now().UTC()
	product, err := saveSquadProductTx(ctx, tx, input, now)
	if err != nil {
		return model.SquadProduct{}, err
	}
	if err = catalogAdminAuditTx(ctx, tx, actorID, "squad_product.save", "squad_product", product.ID, map[string]string{"name": input.Name}, now); err != nil {
		return model.SquadProduct{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.SquadProduct{}, err
	}
	return product, nil
}

func catalogAdminActorTx(ctx context.Context, tx *sql.Tx, actorID string) error {
	if strings.TrimSpace(actorID) == "" {
		return ErrConflict
	}
	var role, status string
	var blocked bool
	err := tx.QueryRowContext(ctx, `SELECT role,account_status,bot_blocked=1 FROM users WHERE id=$1 FOR SHARE`, actorID).Scan(&role, &status, &blocked)
	if errors.Is(err, sql.ErrNoRows) || err == nil && (role != "admin" || status != "active" || blocked) {
		return ErrConflict
	}
	return err
}

func catalogAdminAuditTx(ctx context.Context, tx *sql.Tx, actor, action, targetType, targetID string, detail any, now time.Time) error {
	id, err := ids.New()
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	return insertAuditTx(ctx, tx, id, &actor, action, targetType, targetID, string(encoded), now)
}
