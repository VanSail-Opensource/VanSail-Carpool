package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
	"strings"
	"time"
)

// QueueQuestionnaireSettlement confirms an analyzed import and appends durable work.
func (s *Store) retryAttempt_QueueQuestionnaireSettlement(ctx context.Context, importID string, now time.Time) (questionnaires.ImportPreview, error) {
	if strings.TrimSpace(importID) == "" {
		return questionnaires.ImportPreview{}, questionnaires.ErrInvalidInput
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return questionnaires.ImportPreview{}, err
	}
	defer func() { _ = tx.Rollback() }()
	preview, err := importByIDTx(ctx, tx, importID)
	if err != nil {
		return questionnaires.ImportPreview{}, err
	}
	if preview.Status == questionnaires.ImportStatusQueued || preview.Status == questionnaires.ImportStatusProcessing || preview.Status == questionnaires.ImportStatusSettled {
		return preview, nil
	}
	if preview.Status != questionnaires.ImportStatusPreview {
		return questionnaires.ImportPreview{}, ErrConflict
	}
	if preview.CodeColumn == nil || preview.Analysis == nil || preview.Analysis.CodeColumn != *preview.CodeColumn {
		return questionnaires.ImportPreview{}, ErrConflict
	}
	var competing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM questionnaire_imports WHERE questionnaire_id=? AND id<>? AND status IN ('queued','processing','settled')`, preview.QuestionnaireID, importID).Scan(&competing); err != nil {
		return questionnaires.ImportPreview{}, err
	}
	if competing != 0 {
		return questionnaires.ImportPreview{}, ErrConflict
	}
	now = now.UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE questionnaire_imports SET status='queued',updated_at=? WHERE id=? AND status='preview'`, stamp(now), importID); err != nil {
		return questionnaires.ImportPreview{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE questionnaires SET status='settling',updated_at=? WHERE id=? AND status IN ('active','closed')`, stamp(now), preview.QuestionnaireID)
	if err != nil {
		return questionnaires.ImportPreview{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return questionnaires.ImportPreview{}, rowsErr
	} else if affected != 1 {
		return questionnaires.ImportPreview{}, ErrConflict
	}
	payload, err := json.Marshal(struct {
		ImportID string `json:"importId"`
	}{ImportID: importID})
	if err != nil {
		return questionnaires.ImportPreview{}, err
	}
	if err := insertOutboxTx(ctx, tx, "questionnaire_settlement", string(payload), now, now); err != nil {
		return questionnaires.ImportPreview{}, err
	}
	if err := tx.Commit(); err != nil {
		return questionnaires.ImportPreview{}, err
	}
	return s.questionnaireImportByID(ctx, importID)
}

// SettleQuestionnaireImport atomically matches codes and credits each participant once.
func (s *Store) retryAttempt_SettleQuestionnaireImport(ctx context.Context, importID string, now time.Time) (questionnaires.SettlementReport, error) {
	if strings.TrimSpace(importID) == "" {
		return questionnaires.SettlementReport{}, questionnaires.ErrInvalidInput
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return questionnaires.SettlementReport{}, err
	}
	defer func() { _ = tx.Rollback() }()
	preview, raw, delimiter, reportJSON, err := importSettlementDataTx(ctx, tx, importID)
	if err != nil {
		return questionnaires.SettlementReport{}, err
	}
	if preview.Status == questionnaires.ImportStatusSettled {
		var report questionnaires.SettlementReport
		if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
			return questionnaires.SettlementReport{}, fmt.Errorf("decode questionnaire settlement report: %w", err)
		}
		report.Replayed = true
		return report, nil
	}
	if preview.Status != questionnaires.ImportStatusQueued && preview.Status != questionnaires.ImportStatusProcessing {
		return questionnaires.SettlementReport{}, ErrConflict
	}
	if preview.CodeColumn == nil {
		return questionnaires.SettlementReport{}, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE questionnaire_imports SET status='processing',updated_at=? WHERE id=?`, stamp(now), importID); err != nil {
		return questionnaires.SettlementReport{}, err
	}
	parsed, err := questionnaires.ExtractCodes(raw, delimiter, preview.Headers, *preview.CodeColumn, 50_000)
	if err != nil {
		return questionnaires.SettlementReport{}, err
	}
	var rewardMinor int64
	if err := tx.QueryRowContext(ctx, `SELECT reward_txb_minor FROM questionnaires WHERE id=? AND status='settling'`, preview.QuestionnaireID).Scan(&rewardMinor); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return questionnaires.SettlementReport{}, ErrConflict
		}
		return questionnaires.SettlementReport{}, err
	}
	report := questionnaires.SettlementReport{ImportID: importID, QuestionnaireID: preview.QuestionnaireID,
		DuplicateCount: parsed.DuplicateCount, MalformedCount: parsed.MalformedCount,
		RewardTXBMinor: rewardMinor, SettledAt: now}
	for _, code := range parsed.Codes {
		participant, loadErr := participantByCodeTx(ctx, tx, preview.QuestionnaireID, code)
		if errors.Is(loadErr, ErrNotFound) {
			report.UnknownCount++
			continue
		}
		if loadErr != nil {
			return questionnaires.SettlementReport{}, loadErr
		}
		report.MatchedCount++
		if participant.AwardedAt != nil {
			report.AlreadyAwardedCount++
			continue
		}
		result, updateErr := tx.ExecContext(ctx, `UPDATE questionnaire_participants SET awarded_at=? WHERE id=? AND awarded_at IS NULL`, stamp(now), participant.ID)
		if updateErr != nil {
			return questionnaires.SettlementReport{}, updateErr
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return questionnaires.SettlementReport{}, rowsErr
		} else if affected != 1 {
			report.AlreadyAwardedCount++
			continue
		}
		balance, balanceErr := changeBalanceTx(ctx, tx, participant.UserID, rewardMinor, now)
		if balanceErr != nil {
			return questionnaires.SettlementReport{}, balanceErr
		}
		if _, ledgerErr := insertLedgerTx(ctx, tx, participant.UserID, rewardMinor, balance, "questionnaire_reward", participant.ID, preview.QuestionnaireID, now); ledgerErr != nil {
			return questionnaires.SettlementReport{}, ledgerErr
		}
		report.RewardedCount++
	}
	reportBytes, err := json.Marshal(report)
	if err != nil {
		return questionnaires.SettlementReport{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE questionnaire_imports SET status='settled',raw_csv=CAST('' AS BLOB),sample_rows_json='[]',report_json=?,last_error='',updated_at=? WHERE id=?`, string(reportBytes), stamp(now), importID); err != nil {
		return questionnaires.SettlementReport{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE questionnaires SET status='settled',updated_at=? WHERE id=? AND status='settling'`, stamp(now), preview.QuestionnaireID)
	if err != nil {
		return questionnaires.SettlementReport{}, err
	}
	if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
		return questionnaires.SettlementReport{}, rowsErr
	} else if affected != 1 {
		return questionnaires.SettlementReport{}, ErrConflict
	}
	if err := tx.Commit(); err != nil {
		return questionnaires.SettlementReport{}, err
	}
	return report, nil
}
