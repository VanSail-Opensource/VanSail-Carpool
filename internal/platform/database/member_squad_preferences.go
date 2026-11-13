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
	rows, err := reader.QueryContext(ctx, `SELECT squad_uuid FROM user_disabled_squads WHERE user_id=$1 ORDER BY squad_uuid`, userID)
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

// BeginSquadSwitch checks ownership and access under the shared write transaction.
func (s *Store) retryAttempt_BeginSquadSwitch(ctx context.Context, input providerops.CreateInput, purchaseID, uuid string, enabled bool, now time.Time) (providerops.Operation, error) {
	input, err := providerops.NormalizeCreate(input)
	if err != nil || input.Kind != purchaseops.OperationSquadSwitch || input.ActorUserID != input.OwnerUserID || len(input.Items) != 1 || input.Items[0].TargetType != "purchase" || input.Items[0].TargetID != purchaseID {
		return providerops.Operation{}, ErrConflict
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize preferences with other owner controls before replay/ownership reads.
	if _, err := balanceTx(ctx, tx, input.OwnerUserID); err != nil {
		return providerops.Operation{}, err
	}
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
		_, err = tx.ExecContext(ctx, `DELETE FROM user_disabled_squads WHERE user_id=$1 AND squad_uuid=$2`, input.OwnerUserID, uuid)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT   INTO user_disabled_squads(user_id,squad_uuid) VALUES($1,$2) ON CONFLICT DO NOTHING `, input.OwnerUserID, uuid)
	}
	if err != nil {
		return providerops.Operation{}, err
	}
	operation, _, err := createProviderOperationStateTx(ctx, tx, input, now, true)
	if err != nil {
		return providerops.Operation{}, err
	}
	if err := auditComboControlTx(ctx, tx, input.OwnerUserID, "member.squad_switch", purchaseID, map[string]any{"squadUuid": uuid, "enabled": enabled, "operationId": operation.Receipt.ID}, now); err != nil {
		return providerops.Operation{}, err
	}
	return operation, tx.Commit()
}

// LatestComboControl resumes unresolved commands and failures that still affect access.
func (s *Store) LatestComboControl(ctx context.Context, userID string) (*model.OperationReceipt, error) {
	now := stamp(time.Now().UTC())
	operation, err := scanProviderOperation(s.db.QueryRowContext(ctx, providerOperationSelect+` operation
 WHERE operation.owner_user_id=$1 AND operation.kind IN ($2,$3)
 AND (operation.status IN ('queued','processing','pending_review','partial') OR
 (operation.status='failed' AND (`+liveComboControlPredicate(4)+`)
 AND NOT EXISTS (SELECT 1 FROM provider_operations newer WHERE newer.owner_user_id=operation.owner_user_id
 AND newer.kind IN ($2,$3) AND newer.status IN ('succeeded','compensated')
 AND newer.record_order>operation.record_order)))
 ORDER BY CASE WHEN operation.status='failed' THEN 1 ELSE 0 END,operation.record_order DESC LIMIT 1`,
		userID, purchaseops.OperationSquadSwitch, purchaseops.OperationEarlyActivation, now, now, now, now))
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return &operation.Receipt, err
}
