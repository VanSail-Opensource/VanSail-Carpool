package database

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/purchaseops"
)

type squadPreferenceReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func disabledSquadsFrom(ctx context.Context, reader squadPreferenceReader, userID string) ([]string, error) {
	rows, err := reader.QueryContext(ctx, `SELECT squad_uuid FROM user_disabled_squads WHERE user_id=? ORDER BY squad_uuid`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := []string{}
	for rows.Next() {
		var uuid string
		if err := rows.Scan(&uuid); err != nil {
			return nil, err
		}
		result = append(result, uuid)
	}
	return result, rows.Err()
}

// UserDisabledSquads returns references without duplicating upstream catalog data.
func (s *Store) UserDisabledSquads(ctx context.Context, userID string) ([]string, error) {
	return disabledSquadsFrom(ctx, s.db, userID)
}

// EnabledSquadsForRemote applies account preferences immediately before provider execution.
// A new term whose owned squads are all disabled clears the stable first preference.
func (s *Store) EnabledSquadsForRemote(ctx context.Context, remoteID string, owned []string) ([]string, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var userID string
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE remna_user_id=?`, remoteID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return append([]string{}, owned...), nil
	}
	if err != nil {
		return nil, err
	}
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
	return enabled, tx.Commit()
}

// BeginSquadSwitch checks ownership and access under the shared write transaction.
func (s *Store) BeginSquadSwitch(ctx context.Context, input providerops.CreateInput, purchaseID, uuid string, enabled bool, now time.Time) (providerops.Operation, error) {
	input, err := providerops.NormalizeCreate(input)
	if err != nil || input.Kind != purchaseops.OperationSquadSwitch || input.ActorUserID != input.OwnerUserID || len(input.Items) != 1 || input.Items[0].TargetType != "purchase" || input.Items[0].TargetID != purchaseID {
		return providerops.Operation{}, ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if operation, found, err := memberOperationReplayTx(ctx, tx, input, now); found || err != nil {
		if err == nil {
			err = tx.Commit()
		}
		return operation, err
	}
	purchase, err := activeControlPurchaseTx(ctx, tx, input.OwnerUserID, purchaseID, now)
	if err != nil {
		return providerops.Operation{}, err
	}
	owned, err := purchaseSquadsFrom(ctx, tx, purchase.ID)
	if err != nil {
		return providerops.Operation{}, err
	}
	if !slices.Contains(owned, uuid) {
		return providerops.Operation{}, ErrNotFound
	}
	if conflict, err := comboControlConflictFrom(ctx, tx, input.OwnerUserID); err != nil || conflict {
		if err == nil {
			err = ErrConflict
		}
		return providerops.Operation{}, err
	}
	disabled, err := disabledSquadsFrom(ctx, tx, input.OwnerUserID)
	if err != nil {
		return providerops.Operation{}, err
	}
	effective := purchaseops.EnabledSquads(owned, disabled)
	if !enabled && slices.Contains(effective, uuid) && len(effective) <= 1 {
		return providerops.Operation{}, purchaseops.ErrLastEnabledSquad
	}
	if enabled {
		_, err = tx.ExecContext(ctx, `DELETE FROM user_disabled_squads WHERE user_id=? AND squad_uuid=?`, input.OwnerUserID, uuid)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO user_disabled_squads(user_id,squad_uuid) VALUES(?,?)`, input.OwnerUserID, uuid)
	}
	if err != nil {
		return providerops.Operation{}, err
	}
	operation, _, err := createProviderOperationTx(ctx, tx, input, now)
	if err != nil {
		return providerops.Operation{}, err
	}
	if err := auditComboControlTx(ctx, tx, input.OwnerUserID, "member.squad_switch", purchaseID, map[string]any{"squadUuid": uuid, "enabled": enabled, "operationId": operation.Receipt.ID}, now); err != nil {
		return providerops.Operation{}, err
	}
	return operation, tx.Commit()
}

// LatestComboControl resumes pending/review/failed member commands after a reload.
func (s *Store) LatestComboControl(ctx context.Context, userID string) (*model.OperationReceipt, error) {
	operation, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+` WHERE owner_user_id=? AND kind IN (?,?)
 AND status IN ('queued','processing','failed','pending_review','partial') ORDER BY created_at DESC,id DESC LIMIT 1`, userID, purchaseops.OperationSquadSwitch, purchaseops.OperationEarlyActivation))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return &operation.Receipt, err
}
