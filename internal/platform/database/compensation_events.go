package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
)

const compensationEventSelect = `SELECT id,node_uuid,node_name,status,offline_observed_at,recovered_observed_at,
	observed_duration_seconds,threshold_minutes,multiplier_bps,proposed_extension_minutes,final_extension_minutes,capped,
	frozen_recipient_count,eligible_recipient_count,skipped_recipient_count,reviewed_by,reviewed_at,review_reason,
	ineligible_reason,revision,provider_operation_id FROM node_compensation_events`

// ListCompensationEvents returns cursor-paginated public projections without recipient identities.
func (s *Store) ListCompensationEvents(ctx context.Context, status, cursor string, limit int) (compensation.EventPage, error) {
	query := compensationEventSelect
	args := make([]any, 0, 3)
	where := " WHERE 1=1"
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, pageFilterFingerprint(status))
		if err != nil {
			return compensation.EventPage{}, err
		}
		where += ` AND (created_at<? OR (created_at=? AND id<?))`
		args = append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID)
	}
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query+where+` ORDER BY created_at DESC,id DESC LIMIT ?`, args...)
	if err != nil {
		return compensation.EventPage{}, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]compensation.Event, 0, limit+1)
	for rows.Next() {
		event, scanErr := scanCompensationEvent(rows)
		if scanErr != nil {
			return compensation.EventPage{}, scanErr
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return compensation.EventPage{}, err
	}
	var next *string
	if len(items) > limit {
		last := items[limit-1]
		value, err := encodeTimestampCursor(last.OfflineObservedAt, last.ID, pageFilterFingerprint(status))
		if err != nil {
			return compensation.EventPage{}, err
		}
		next = &value
		items = items[:limit]
	}
	for index := range items {
		if err := s.hydrateCompensationEvent(ctx, &items[index]); err != nil {
			return compensation.EventPage{}, err
		}
	}
	return compensation.EventPage{Items: items, NextCursor: next}, nil
}

func (s *Store) compensationEventByID(ctx context.Context, eventID string) (compensation.Event, error) {
	event, err := scanCompensationEvent(s.db.QueryRowContext(ctx, compensationEventSelect+` WHERE id=?`, eventID))
	if err == nil {
		err = s.hydrateCompensationEvent(ctx, &event)
	}
	return event, err
}

func scanCompensationEvent(row rowScanner) (compensation.Event, error) {
	var event compensation.Event
	var seconds, proposed, final sql.NullInt64
	var recovered, reviewer, reviewed, reason, ineligible, operation sql.NullString
	var offline string
	var capped int
	err := row.Scan(&event.ID, &event.NodeUUID, &event.NodeName, &event.Status, &offline, &recovered, &seconds,
		&event.ThresholdMinutes, &event.MultiplierBPS, &proposed, &final, &capped, &event.FrozenRecipientCount,
		&event.EligibleRecipientCount, &event.SkippedRecipientCount, &reviewer, &reviewed, &reason, &ineligible,
		&event.Revision, &operation)
	if errors.Is(err, sql.ErrNoRows) {
		return event, ErrNotFound
	}
	if err != nil {
		return event, fmt.Errorf("scan compensation event: %w", err)
	}
	event.Capped = capped == 1
	event.ReviewedBy, event.ReviewReason, event.IneligibleReason = nullableString(reviewer), nullableString(reason), nullableString(ineligible)
	if event.OfflineObservedAt, err = parseStamp(offline); err != nil {
		return event, err
	}
	event.RecoveredObservedAt, err = parseOptionalStamp(recovered)
	if err == nil {
		event.ReviewedAt, err = parseOptionalStamp(reviewed)
	}
	setNullableNumbers(&event, seconds, proposed, final)
	_ = operation
	return event, err
}

func setNullableNumbers(event *compensation.Event, seconds, proposed, final sql.NullInt64) {
	if seconds.Valid {
		value := seconds.Int64
		event.ObservedDurationSeconds = &value
	}
	if proposed.Valid {
		value := int(proposed.Int64)
		event.ProposedExtensionMinutes = &value
	}
	if final.Valid {
		value := int(final.Int64)
		event.FinalExtensionMinutes = &value
	}
}

func (s *Store) hydrateCompensationEvent(ctx context.Context, event *compensation.Event) error {
	event.Squads = []compensation.Squad{}
	rows, err := s.db.QueryContext(ctx, `SELECT squad_uuid,squad_name FROM node_compensation_event_squads WHERE event_id=? ORDER BY squad_name,squad_uuid`, event.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var squad compensation.Squad
		if err := rows.Scan(&squad.UUID, &squad.Name); err != nil {
			_ = rows.Close()
			return err
		}
		event.Squads = append(event.Squads, squad)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	var operationID sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT provider_operation_id FROM node_compensation_events WHERE id=?`, event.ID).Scan(&operationID); err != nil {
		return err
	}
	if operationID.Valid {
		operation, err := s.ProviderOperationByID(ctx, operationID.String)
		if err != nil {
			return err
		}
		event.Operation = &operation.Receipt
	}
	return nil
}
