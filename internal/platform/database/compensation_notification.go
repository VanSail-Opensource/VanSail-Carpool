package database

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func applyCompensationExtension(ctx context.Context, store *Store, tx *sql.Tx, event compensation.Event,
	operationID string, targets []adminBulkTarget, minutes int, reason string, now time.Time) error {
	baseFacts, err := compensationNotificationFacts(ctx, tx, event, minutes, reason, now)
	if err != nil {
		return err
	}
	for _, target := range targets {
		before, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=?`, target.PurchaseID))
		if err != nil {
			return err
		}
		if err := shiftAdminSubscriptionTx(ctx, tx, target, minutes, now); err != nil {
			return err
		}
		newExpiry, err := addSubscriptionMinutes(before.ValidUntil, minutes)
		if err != nil {
			return err
		}
		facts := cloneNotificationFacts(baseFacts)
		facts[notifications.FactPreviousExpiry] = before.ValidUntil.Format(time.RFC3339Nano)
		facts[notifications.FactNewExpiry] = newExpiry.Format(time.RFC3339Nano)
		facts[notifications.FactCombo] = before.ComboName
		if _, err := store.insertUserNotificationTx(ctx, tx, "compensation:"+operationID+":"+target.UserID, target.UserID,
			jobpayload.UserEventNodeCompensation, providerItemGate(operationID, target.UserID), facts, now); err != nil {
			return err
		}
	}
	return nil
}

func compensationNotificationFacts(ctx context.Context, tx *sql.Tx, event compensation.Event, minutes int,
	reason string, now time.Time) (map[string]string, error) {
	if event.RecoveredObservedAt == nil || event.ObservedDurationSeconds == nil {
		return nil, errors.New("compensation recovery snapshot is incomplete")
	}
	squads, err := compensationSquadNames(ctx, tx, event.ID)
	if err != nil {
		return nil, err
	}
	facts := map[string]string{
		notifications.FactNode:            event.NodeName,
		notifications.FactAffectedSquads:  strings.Join(squads, ", "),
		notifications.FactOutageStarted:   event.OfflineObservedAt.Format(time.RFC3339Nano),
		notifications.FactRecovered:       event.RecoveredObservedAt.Format(time.RFC3339Nano),
		notifications.FactDowntimeSeconds: strconv.FormatInt(*event.ObservedDurationSeconds, 10),
		notifications.FactAddedSeconds:    strconv.FormatInt(int64(minutes)*60, 10),
		notifications.FactReason:          strings.TrimSpace(reason),
		notifications.FactTime:            now.UTC().Format(time.RFC3339Nano),
	}
	if event.Capped {
		facts[notifications.FactCompensationCapped] = "true"
	}
	return facts, nil
}

func compensationSquadNames(ctx context.Context, tx *sql.Tx, eventID string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT squad_name FROM node_compensation_event_squads
		WHERE event_id=? ORDER BY squad_name,squad_uuid`, eventID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	names := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, errors.New("compensation squad snapshot is empty")
	}
	return names, nil
}

func cloneNotificationFacts(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source)+3)
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
