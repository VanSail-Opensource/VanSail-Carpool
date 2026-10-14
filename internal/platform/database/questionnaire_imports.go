package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
	"strings"
	"time"
)

// CreateQuestionnaireImport persists a bounded preview without applying rewards.
func (s *Store) CreateQuestionnaireImport(ctx context.Context, questionnaireID string, document questionnaires.CSVDocument, idempotencyKey string, now time.Time) (questionnaires.ImportPreview, error) {
	if strings.TrimSpace(questionnaireID) == "" || strings.TrimSpace(idempotencyKey) == "" || len(idempotencyKey) > 128 || len(document.Raw) == 0 || len(document.Raw) > 5<<20 || len(document.Headers) == 0 || len(document.Headers) > 100 || document.DataRowCount > 50_000 {
		return questionnaires.ImportPreview{}, questionnaires.ErrInvalidInput
	}
	if document.Delimiter != ',' && document.Delimiter != ';' && document.Delimiter != '\t' {
		return questionnaires.ImportPreview{}, questionnaires.ErrInvalidInput
	}
	headersJSON, err := json.Marshal(document.Headers)
	if err != nil {
		return questionnaires.ImportPreview{}, err
	}
	sampleJSON, err := json.Marshal(document.SampleRows)
	if err != nil {
		return questionnaires.ImportPreview{}, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return questionnaires.ImportPreview{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, loadErr := importByKeyTx(ctx, tx, questionnaireID, idempotencyKey); loadErr == nil {
		return existing, nil
	} else if !errors.Is(loadErr, ErrNotFound) {
		return questionnaires.ImportPreview{}, loadErr
	}
	var status questionnaires.Status
	if err := tx.QueryRowContext(ctx, `SELECT status FROM questionnaires WHERE id=?`, questionnaireID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return questionnaires.ImportPreview{}, ErrNotFound
		}
		return questionnaires.ImportPreview{}, err
	}
	if status != questionnaires.StatusActive && status != questionnaires.StatusClosed {
		return questionnaires.ImportPreview{}, ErrConflict
	}
	importID, err := ids.New()
	if err != nil {
		return questionnaires.ImportPreview{}, err
	}
	now = now.UTC()
	_, err = tx.ExecContext(ctx, `INSERT INTO questionnaire_imports(id,questionnaire_id,status,raw_csv,delimiter,headers_json,sample_rows_json,data_row_count,malformed_row_count,idempotency_key,created_at,updated_at)
		VALUES(?,?,'preview',?,?,?,?,?,?,?,?,?)`, importID, questionnaireID, document.Raw, string(document.Delimiter), string(headersJSON), string(sampleJSON),
		document.DataRowCount, document.MalformedRowCount, idempotencyKey, stamp(now), stamp(now))
	if err != nil {
		return questionnaires.ImportPreview{}, fmt.Errorf("create questionnaire import: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return questionnaires.ImportPreview{}, err
	}
	return s.questionnaireImportByID(ctx, importID)
}

// AnalyzeQuestionnaireImport matches a selected code column without applying rewards.
func (s *Store) AnalyzeQuestionnaireImport(ctx context.Context, importID, codeColumn string, now time.Time) (questionnaires.ImportAnalysis, error) {
	if strings.TrimSpace(importID) == "" || strings.TrimSpace(codeColumn) == "" {
		return questionnaires.ImportAnalysis{}, questionnaires.ErrInvalidInput
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return questionnaires.ImportAnalysis{}, err
	}
	defer func() { _ = tx.Rollback() }()
	preview, raw, delimiter, _, err := importSettlementDataTx(ctx, tx, importID)
	if err != nil {
		return questionnaires.ImportAnalysis{}, err
	}
	if preview.Status != questionnaires.ImportStatusPreview {
		return questionnaires.ImportAnalysis{}, ErrConflict
	}
	matches := 0
	for _, header := range preview.Headers {
		if header == codeColumn {
			matches++
		}
	}
	if matches != 1 {
		return questionnaires.ImportAnalysis{}, questionnaires.ErrInvalidInput
	}
	parsed, err := questionnaires.ExtractCodes(raw, delimiter, preview.Headers, codeColumn, 50_000)
	if err != nil {
		return questionnaires.ImportAnalysis{}, err
	}
	analysis := questionnaires.ImportAnalysis{ImportID: importID, QuestionnaireID: preview.QuestionnaireID, CodeColumn: codeColumn,
		DuplicateCount: parsed.DuplicateCount, MalformedCount: parsed.MalformedCount}
	for _, code := range parsed.Codes {
		participant, loadErr := participantByCodeTx(ctx, tx, preview.QuestionnaireID, code)
		if errors.Is(loadErr, ErrNotFound) {
			analysis.UnknownCount++
			continue
		}
		if loadErr != nil {
			return questionnaires.ImportAnalysis{}, loadErr
		}
		analysis.MatchedCount++
		if participant.AwardedAt != nil {
			analysis.AlreadyAwardedCount++
		}
	}
	analysisJSON, err := json.Marshal(analysis)
	if err != nil {
		return questionnaires.ImportAnalysis{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE questionnaire_imports SET code_column=?,analysis_json=?,updated_at=? WHERE id=? AND status='preview'`, codeColumn, string(analysisJSON), stamp(now.UTC()), importID); err != nil {
		return questionnaires.ImportAnalysis{}, err
	}
	if err := tx.Commit(); err != nil {
		return questionnaires.ImportAnalysis{}, err
	}
	return analysis, nil
}
