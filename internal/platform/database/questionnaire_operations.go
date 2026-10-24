package database

import (
	"context"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

// BeginQuestionnaireSettlementOperation atomically confirms an import and queues its receipt.
func (s *Store) retryAttempt_BeginQuestionnaireSettlementOperation(ctx context.Context, input providerops.CreateInput,
	importID string, now time.Time) (providerops.Operation, bool, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	operation, replayed, err := createProviderOperationTx(ctx, tx, input, now.UTC())
	if err != nil || replayed {
		if err == nil {
			err = tx.Commit()
		}
		return operation, replayed, err
	}
	preview, err := importByIDTx(ctx, tx, importID)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if preview.Status != questionnaires.ImportStatusPreview || preview.CodeColumn == nil || preview.Analysis == nil ||
		preview.Analysis.CodeColumn != *preview.CodeColumn {
		return providerops.Operation{}, false, ErrConflict
	}
	var competing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM questionnaire_imports WHERE questionnaire_id=? AND id<>?
		AND status IN ('queued','processing','settled')`, preview.QuestionnaireID, importID).Scan(&competing); err != nil || competing != 0 {
		if err == nil {
			err = ErrConflict
		}
		return providerops.Operation{}, false, err
	}
	stampNow := stamp(now.UTC())
	if _, err := tx.ExecContext(ctx, `UPDATE questionnaire_imports SET status='queued',updated_at=? WHERE id=? AND status='preview'`, stampNow, importID); err != nil {
		return providerops.Operation{}, false, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE questionnaires SET status='settling',updated_at=? WHERE id=? AND status IN ('active','closed')`, stampNow, preview.QuestionnaireID)
	if err != nil {
		return providerops.Operation{}, false, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil || affected != 1 {
		if rowsErr != nil {
			return providerops.Operation{}, false, rowsErr
		}
		return providerops.Operation{}, false, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return providerops.Operation{}, false, fmt.Errorf("commit questionnaire settlement operation: %w", err)
	}
	return operation, false, nil
}
