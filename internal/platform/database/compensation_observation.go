package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// RecordCompensationObservation atomically advances only nodes present in a complete sample.
func (s *Store) RecordCompensationObservation(ctx context.Context, config compensation.Config, nodes []compensation.Node, now time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, node := range nodes {
		if node.UUID == "" || node.Name == "" {
			return compensation.ErrInvalid
		}
		eventID, loadErr := observingCompensationEvent(ctx, tx, node.UUID)
		if loadErr != nil && !errors.Is(loadErr, ErrNotFound) {
			return loadErr
		}
		switch {
		case node.Disabled && loadErr == nil:
			err = closeCompensationEvent(ctx, tx, eventID, now, "node_disabled")
		case node.Connected && loadErr == nil:
			err = recoverCompensationEvent(ctx, tx, eventID, now)
		case !node.Connected && !node.Disabled && errors.Is(loadErr, ErrNotFound) && config.Enabled:
			eventID, err = startCompensationEvent(ctx, tx, config, node, now)
		case errors.Is(loadErr, ErrNotFound):
			eventID = ""
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO node_compensation_node_state(node_uuid,node_name,is_connected,is_disabled,
			open_event_id,last_observed_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(node_uuid) DO UPDATE SET
			node_name=excluded.node_name,is_connected=excluded.is_connected,is_disabled=excluded.is_disabled,
			open_event_id=excluded.open_event_id,last_observed_at=excluded.last_observed_at,updated_at=excluded.updated_at`,
			node.UUID, node.Name, boolInt(node.Connected), boolInt(node.Disabled), nullIfEmpty(eventID), stamp(now), stamp(now)); err != nil {
			return fmt.Errorf("save compensation node state: %w", err)
		}
	}
	return tx.Commit()
}

func observingCompensationEvent(ctx context.Context, tx *sql.Tx, nodeUUID string) (string, error) {
	var eventID string
	err := tx.QueryRowContext(ctx, `SELECT id FROM node_compensation_events WHERE node_uuid=? AND status='observing'`, nodeUUID).Scan(&eventID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return eventID, err
}

func startCompensationEvent(ctx context.Context, tx *sql.Tx, config compensation.Config, node compensation.Node, now time.Time) (string, error) {
	if config.ThresholdMinutes == nil || config.MultiplierBPS == nil {
		return "", compensation.ErrInvalid
	}
	eventID, err := ids.New()
	if err != nil {
		return "", err
	}
	squadIDs := make([]string, 0, len(node.AffectedSquads))
	for _, squad := range node.AffectedSquads {
		if squad.UUID == "" || squad.Name == "" {
			return "", compensation.ErrInvalid
		}
		squadIDs = append(squadIDs, squad.UUID)
	}
	targets := []adminBulkTarget{}
	if len(squadIDs) > 0 {
		targets, _, err = adminBulkTargets(ctx, tx, AdminBulkExtensionFilter{AddonSquadUUIDs: squadIDs}, now)
		if err != nil {
			return "", err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO node_compensation_events(id,node_uuid,node_name,status,offline_observed_at,
		threshold_minutes,multiplier_bps,frozen_recipient_count,revision,created_at,updated_at)
		VALUES(?,?,?,'observing',?,?,?,?,0,?,?)`, eventID, node.UUID, node.Name, stamp(now), *config.ThresholdMinutes,
		*config.MultiplierBPS, len(targets), stamp(now), stamp(now))
	if err != nil {
		return "", fmt.Errorf("start compensation event: %w", err)
	}
	for _, squad := range node.AffectedSquads {
		if _, err := tx.ExecContext(ctx, `INSERT INTO node_compensation_event_squads(event_id,squad_uuid,squad_name) VALUES(?,?,?)`,
			eventID, squad.UUID, squad.Name); err != nil {
			return "", err
		}
	}
	for _, target := range targets {
		if _, err := tx.ExecContext(ctx, `INSERT INTO node_compensation_event_recipients(event_id,user_id) VALUES(?,?)`, eventID, target.UserID); err != nil {
			return "", err
		}
	}
	return eventID, nil
}

func recoverCompensationEvent(ctx context.Context, tx *sql.Tx, eventID string, now time.Time) error {
	var offline string
	var threshold, multiplier, recipients int
	if err := tx.QueryRowContext(ctx, `SELECT offline_observed_at,threshold_minutes,multiplier_bps,frozen_recipient_count
		FROM node_compensation_events WHERE id=? AND status='observing'`, eventID).Scan(&offline, &threshold, &multiplier, &recipients); err != nil {
		return err
	}
	started, err := parseStamp(offline)
	if err != nil {
		return err
	}
	if now.Before(started) {
		return compensation.ErrConflict
	}
	seconds := int64(now.Sub(started) / time.Second)
	minutes, capped, reason := compensation.RecoveryOutcome(seconds, threshold, multiplier, recipients)
	status := "pending_review"
	var reasonValue any
	if reason != "" {
		status, reasonValue = "ineligible", reason
	}
	_, err = tx.ExecContext(ctx, `UPDATE node_compensation_events SET status=?,recovered_observed_at=?,observed_duration_seconds=?,
		proposed_extension_minutes=?,capped=?,ineligible_reason=?,revision=revision+1,updated_at=? WHERE id=? AND status='observing'`,
		status, stamp(now), seconds, minutes, boolInt(capped), reasonValue, stamp(now), eventID)
	return err
}

func closeCompensationEvent(ctx context.Context, tx *sql.Tx, eventID string, now time.Time, reason string) error {
	_, err := tx.ExecContext(ctx, `UPDATE node_compensation_events SET status='ineligible',recovered_observed_at=?,
		observed_duration_seconds=CAST((julianday(?) - julianday(offline_observed_at))*86400 AS INTEGER),ineligible_reason=?,
		revision=revision+1,updated_at=? WHERE id=? AND status='observing'`, stamp(now), stamp(now), reason, stamp(now), eventID)
	return err
}
