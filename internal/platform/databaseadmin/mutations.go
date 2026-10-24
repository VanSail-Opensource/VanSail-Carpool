package databaseadmin

import (
	"context"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"regexp"
	"strings"
	"time"
)

var decimalPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)$`)

type preparedMutation struct {
	schema         tableSchema
	current        *rowData
	columns        []schemaColumn
	arguments      []any
	afterRaw       []any
	changedColumns []string
}

// ReviewMutation creates a one-use, ten-minute review for the exact redacted
// diff. Review tokens are persisted so an insert cannot be replayed after a
// process restart.

func (s *Service) ReviewMutation(ctx context.Context, actorUserID string, request MutationRequest) (MutationReview, error) {
	if s.postgresReadOnly() {
		return MutationReview{}, fmt.Errorf("%w: PostgreSQL generic mutations are disabled pending parity verification", ErrInvalidValue)
	}
	request.ReviewHash = ""
	request.Confirmation = ""
	if err := validateAction(request.Action); err != nil {
		return MutationReview{}, err
	}
	reason, err := normalizeReason(request.Reason)
	if err != nil {
		return MutationReview{}, err
	}
	request.Reason = reason
	schema, err := s.schema(ctx, request.Table)
	if err != nil {
		return MutationReview{}, err
	}
	prepared, err := s.prepare(ctx, s.db, schema, request, false)
	if err != nil {
		return MutationReview{}, err
	}
	digest, err := mutationDigest(actorUserID, request)
	if err != nil {
		return MutationReview{}, err
	}
	token, err := ids.Token(32)
	if err != nil {
		return MutationReview{}, fmt.Errorf("create database review token: %w", err)
	}
	now := s.now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MutationReview{}, fmt.Errorf("begin database review: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM database_admin_reviews WHERE consumed_at IS NOT NULL OR expires_at<?`, now.Format(time.RFC3339Nano)); err != nil {
		return MutationReview{}, fmt.Errorf("prune database reviews: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO database_admin_reviews(id,actor_user_id,action,table_name,request_hash,expires_at,created_at) VALUES(?,?,?,?,?,?,?)`,
		token, actorUserID, request.Action, schema.Name, digest, now.Add(10*time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		return MutationReview{}, fmt.Errorf("store database review: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MutationReview{}, fmt.Errorf("commit database review: %w", err)
	}
	review := MutationReview{
		Action: request.Action, Table: schema.Name, Key: cloneKey(request.Key), ChangedColumns: prepared.changedColumns,
		ReviewHash: token, RequiredConfirmation: requiredConfirmation(request.Action, schema.Name),
		RescueBackupRequired: true, Warning: bypassWarning,
	}
	if prepared.current != nil {
		review.Before = cloneValues(prepared.current.record.Values)
	}
	if request.Action != "delete" {
		keyRaw := keyRawFromRecord(schema, prepared.afterRaw, prepared.current)
		record, err := s.makeRecord(schema, prepared.afterRaw, keyRaw)
		if err != nil {
			return MutationReview{}, err
		}
		review.After = record.Values
		if request.Action == "insert" {
			// Omitted insert columns are populated by SQLite defaults at apply
			// time. Do not misrepresent them as NULL in the review; show only
			// the explicit typed values the administrator is submitting.
			for _, column := range schema.ColumnsRaw {
				if _, supplied := request.Values[column.Name]; !supplied {
					delete(review.After, column.Name)
				}
			}
		}
	}
	return review, nil
}

// ApplyMutation consumes an exact review, creates a rescue backup, rechecks the
// optimistic record hash, and performs the edit plus audit in one transaction.

func (s *Service) ApplyMutation(ctx context.Context, actorUserID string, request MutationRequest) (MutationResult, error) {
	if s.postgresReadOnly() {
		return MutationResult{}, fmt.Errorf("%w: PostgreSQL generic mutations are disabled pending parity verification", ErrInvalidValue)
	}
	s.mutation.Lock()
	defer s.mutation.Unlock()
	if err := validateAction(request.Action); err != nil {
		return MutationResult{}, err
	}
	reason, err := normalizeReason(request.Reason)
	if err != nil {
		return MutationResult{}, err
	}
	request.Reason = reason
	if request.Confirmation != requiredConfirmation(request.Action, request.Table) {
		return MutationResult{}, ErrConfirmation
	}
	if strings.TrimSpace(request.ReviewHash) == "" {
		return MutationResult{}, ErrReviewConflict
	}
	schema, err := s.schema(ctx, request.Table)
	if err != nil {
		return MutationResult{}, err
	}
	digest, err := mutationDigest(actorUserID, request)
	if err != nil {
		return MutationResult{}, err
	}
	if err := s.checkReview(ctx, actorUserID, request, digest); err != nil {
		return MutationResult{}, err
	}
	if s.backups == nil {
		return MutationResult{}, errors.New("database rescue backup service is unavailable")
	}
	rescue, err := s.backups.Run(ctx)
	if err != nil {
		return MutationResult{}, fmt.Errorf("create database editor rescue backup: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MutationResult{}, fmt.Errorf("begin direct database edit: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := consumeReview(ctx, tx, actorUserID, request, digest, s.now().UTC()); err != nil {
		return MutationResult{}, err
	}
	prepared, err := s.prepare(ctx, tx, schema, request, true)
	if err != nil {
		return MutationResult{}, err
	}
	result, err := s.execute(ctx, tx, request.Action, prepared)
	if err != nil {
		return MutationResult{}, err
	}
	result.RescueBackupID = rescue.ID
	if err := s.auditMutation(ctx, tx, actorUserID, request, prepared, result, rescue.ID); err != nil {
		return MutationResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return MutationResult{}, fmt.Errorf("commit direct database edit: %w", err)
	}
	s.logger.Warn("direct database edit applied",
		"actor_user_id", actorUserID, "table", schema.Name, "action", request.Action,
		"changed_columns", prepared.changedColumns, "reason", "[redacted]", "values", "[redacted]",
		"rescue_backup_id", rescue.ID, "bypasses_domain_hooks", true)
	return result, nil
}

func validateAction(action string) error {
	switch action {
	case "insert", "update", "delete":
		return nil
	default:
		return fmt.Errorf("%w: action must be insert, update, or delete", ErrInvalidValue)
	}
}

func requiredConfirmation(action, table string) string {
	if action == "delete" {
		return "DELETE " + table
	}
	return "EDIT " + table
}
