package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// AdminEntitlementGrantReplay retains the accepted creation result separately
// from the mutable purchase and catalog currently shown in account history.
type AdminEntitlementGrantReplay struct {
	Purchase           model.Purchase
	RequestFingerprint string
}

// AdminEntitlementGrantByKey reads existing immutable creation evidence before
// a service attempts validation against the current catalog or provider.
func (s *Store) AdminEntitlementGrantByKey(ctx context.Context, actorID, userID, key string) (AdminEntitlementGrantReplay, bool, error) {
	if strings.TrimSpace(actorID) == "" || strings.TrimSpace(userID) == "" || strings.TrimSpace(key) == "" {
		return AdminEntitlementGrantReplay{}, false, ErrConflict
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return AdminEntitlementGrantReplay{}, false, err
	}
	defer tx.Rollback()
	return adminEntitlementGrantReplayTx(ctx, tx, actorID, userID, key)
}

func adminEntitlementGrantReplayTx(ctx context.Context, tx *sql.Tx, actorID, userID, key string) (AdminEntitlementGrantReplay, bool, error) {
	var replay AdminEntitlementGrantReplay
	var retainedTerm sql.NullInt64
	key = adminGrantKeyPrefix + actorID + ":" + key
	err := tx.QueryRowContext(ctx, `SELECT id,request_fingerprint,purchased_term_seconds FROM purchases WHERE user_id=$1 AND idempotency_key=$2`, userID, key).
		Scan(&replay.Purchase.ID, &replay.RequestFingerprint, &retainedTerm)
	if errors.Is(err, sql.ErrNoRows) {
		return replay, false, nil
	}
	if err != nil {
		return replay, false, err
	}
	var raw, created string
	err = tx.QueryRowContext(ctx, `SELECT detail,created_at FROM audit_events WHERE actor_user_id=$1 AND action='entitlement.create'
		AND target_type='purchase' AND target_id=$2 ORDER BY created_at,id LIMIT 1`, actorID, replay.Purchase.ID).Scan(&raw, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return replay, true, ErrConflict
	}
	if err != nil {
		return replay, true, err
	}
	var evidence struct {
		TXBMovementMinor *int64 `json:"txbMovementMinor"`
		After            struct {
			ComboID, ComboName, Status, ResetStrategy string
			ValidFrom, ValidUntil                     time.Time
			TrafficLimitBytes                         int64
			SquadUUIDs                                []string
		} `json:"after"`
	}
	if json.Unmarshal([]byte(raw), &evidence) != nil || evidence.TXBMovementMinor == nil || *evidence.TXBMovementMinor != 0 {
		return replay, true, ErrConflict
	}
	after := evidence.After
	if after.ComboID == "" || after.TrafficLimitBytes <= 0 || !after.ValidUntil.After(after.ValidFrom) || (after.Status != "active" && after.Status != "queued") {
		return replay, true, ErrConflict
	}
	if after.ResetStrategy != "DAY" && after.ResetStrategy != "WEEK" && after.ResetStrategy != "MONTH_ROLLING" {
		return replay, true, ErrConflict
	}
	at, err := parseStamp(created)
	if err != nil {
		return replay, true, err
	}
	if retainedTerm.Valid && retainedTerm.Int64 <= 0 {
		return replay, true, ErrConflict
	}
	squads := append([]string{}, after.SquadUUIDs...)
	slices.Sort(squads)
	squads = slices.Compact(squads)
	replay.Purchase = model.Purchase{ID: replay.Purchase.ID, UserID: userID, ComboID: after.ComboID, ComboName: after.ComboName,
		Price: model.TXBMoney(0), GrossPrice: model.TXBMoney(0), CouponDiscount: model.TXBMoney(0),
		ValidFrom: after.ValidFrom, ValidUntil: after.ValidUntil, Status: after.Status,
		TrafficLimitBytes: after.TrafficLimitBytes, TrafficLimit: strconv.FormatInt(after.TrafficLimitBytes, 10),
		ResetStrategy: after.ResetStrategy, PurchasedTermSeconds: int64Pointer(retainedTerm), SquadUUIDs: squads, CreatedAt: at, UpdatedAt: at}
	return replay, true, nil
}

func (s *Store) replayManualGrantInsertConflict(ctx context.Context, input AdminEntitlementCreateInput, cause error) (model.Purchase, error) {
	replay, found, err := s.AdminEntitlementGrantByKey(ctx, input.ActorUserID, input.UserID, input.IdempotencyKey)
	if err != nil {
		return model.Purchase{}, errors.Join(cause, err)
	}
	if !found {
		return model.Purchase{}, cause
	}
	if replay.RequestFingerprint != input.RequestFingerprint {
		return model.Purchase{}, ErrConflict
	}
	return replay.Purchase, nil
}
