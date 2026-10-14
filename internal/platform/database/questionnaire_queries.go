package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

func (s *Store) QuestionnaireImportState(ctx context.Context, importID string) (questionnaires.ImportState, error) {
	preview, err := s.questionnaireImportByID(ctx, importID)
	if err != nil {
		return questionnaires.ImportState{}, err
	}
	state := questionnaires.ImportState{Preview: preview}
	if preview.Status != questionnaires.ImportStatusSettled {
		return state, nil
	}
	var reportJSON string
	if err := s.db.QueryRowContext(ctx, `SELECT report_json FROM questionnaire_imports WHERE id=?`, importID).Scan(&reportJSON); err != nil {
		return questionnaires.ImportState{}, err
	}
	var report questionnaires.SettlementReport
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil {
		return questionnaires.ImportState{}, fmt.Errorf("decode questionnaire settlement report: %w", err)
	}
	state.Report = &report
	return state, nil
}

func questionnaireByID(ctx context.Context, queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, questionnaireID string) (questionnaires.Questionnaire, error) {
	return scanQuestionnaire(queryer.QueryRowContext(ctx, questionnaireSelect+` WHERE q.id=?`, questionnaireID))
}

func scanQuestionnaire(row rowScanner) (questionnaires.Questionnaire, error) {
	var questionnaire questionnaires.Questionnaire
	var closesAt sql.NullString
	var created, updated string
	if err := row.Scan(&questionnaire.ID, &questionnaire.Title, &questionnaire.Description, &questionnaire.FormURL, &questionnaire.RewardTXBMinor,
		&questionnaire.Status, &closesAt, &questionnaire.ParticipantCount, &questionnaire.RewardedCount, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return questionnaires.Questionnaire{}, ErrNotFound
		}
		return questionnaires.Questionnaire{}, err
	}
	var err error
	if questionnaire.CreatedAt, err = parseStamp(created); err != nil {
		return questionnaires.Questionnaire{}, err
	}
	if closesAt.Valid {
		value, parseErr := parseStamp(closesAt.String)
		if parseErr != nil {
			return questionnaires.Questionnaire{}, parseErr
		}
		questionnaire.ClosesAt = &value
	}
	questionnaire.UpdatedAt, err = parseStamp(updated)
	return questionnaire, err
}

const participantSelect = `SELECT id,questionnaire_id,user_id,validation_code,awarded_at,created_at FROM questionnaire_participants`

func (s *Store) questionnaireParticipantByID(ctx context.Context, participantID string) (questionnaires.Participant, error) {
	return scanParticipant(s.db.QueryRowContext(ctx, participantSelect+` WHERE id=?`, participantID))
}

func participantByUserTx(ctx context.Context, tx *sql.Tx, questionnaireID, userID string) (questionnaires.Participant, error) {
	return scanParticipant(tx.QueryRowContext(ctx, participantSelect+` WHERE questionnaire_id=? AND user_id=?`, questionnaireID, userID))
}

func participantByCodeTx(ctx context.Context, tx *sql.Tx, questionnaireID, code string) (questionnaires.Participant, error) {
	return scanParticipant(tx.QueryRowContext(ctx, participantSelect+` WHERE questionnaire_id=? AND validation_code=?`, questionnaireID, questionnaires.NormalizeValidationCode(code)))
}

func scanParticipant(row rowScanner) (questionnaires.Participant, error) {
	var participant questionnaires.Participant
	var awarded sql.NullString
	var created string
	if err := row.Scan(&participant.ID, &participant.QuestionnaireID, &participant.UserID, &participant.ValidationCode, &awarded, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return questionnaires.Participant{}, ErrNotFound
		}
		return questionnaires.Participant{}, err
	}
	var err error
	if participant.CreatedAt, err = parseStamp(created); err != nil {
		return questionnaires.Participant{}, err
	}
	if awarded.Valid {
		value, parseErr := parseStamp(awarded.String)
		if parseErr != nil {
			return questionnaires.Participant{}, parseErr
		}
		participant.AwardedAt = &value
	}
	return participant, nil
}

const questionnaireImportSelect = `SELECT id,questionnaire_id,status,delimiter,headers_json,sample_rows_json,data_row_count,malformed_row_count,code_column,analysis_json,idempotency_key,created_at,updated_at FROM questionnaire_imports`

func (s *Store) questionnaireImportByID(ctx context.Context, importID string) (questionnaires.ImportPreview, error) {
	return scanQuestionnaireImport(s.db.QueryRowContext(ctx, questionnaireImportSelect+` WHERE id=?`, importID))
}

func importByIDTx(ctx context.Context, tx *sql.Tx, importID string) (questionnaires.ImportPreview, error) {
	return scanQuestionnaireImport(tx.QueryRowContext(ctx, questionnaireImportSelect+` WHERE id=?`, importID))
}

func importByKeyTx(ctx context.Context, tx *sql.Tx, questionnaireID, key string) (questionnaires.ImportPreview, error) {
	return scanQuestionnaireImport(tx.QueryRowContext(ctx, questionnaireImportSelect+` WHERE questionnaire_id=? AND idempotency_key=?`, questionnaireID, key))
}

func scanQuestionnaireImport(row rowScanner) (questionnaires.ImportPreview, error) {
	var preview questionnaires.ImportPreview
	var delimiter, headersJSON, samplesJSON string
	var codeColumn, analysisJSON sql.NullString
	var created, updated string
	if err := row.Scan(&preview.ID, &preview.QuestionnaireID, &preview.Status, &delimiter, &headersJSON, &samplesJSON, &preview.DataRowCount,
		&preview.MalformedRowCount, &codeColumn, &analysisJSON, &preview.IdempotencyKey, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return questionnaires.ImportPreview{}, ErrNotFound
		}
		return questionnaires.ImportPreview{}, err
	}
	switch delimiter {
	case ";":
		preview.Delimiter = "semicolon"
	case "\t":
		preview.Delimiter = "tab"
	default:
		preview.Delimiter = "comma"
	}
	if err := json.Unmarshal([]byte(headersJSON), &preview.Headers); err != nil {
		return questionnaires.ImportPreview{}, err
	}
	if err := json.Unmarshal([]byte(samplesJSON), &preview.SampleRows); err != nil {
		return questionnaires.ImportPreview{}, err
	}
	if codeColumn.Valid {
		preview.CodeColumn = &codeColumn.String
	}
	if analysisJSON.Valid {
		var analysis questionnaires.ImportAnalysis
		if err := json.Unmarshal([]byte(analysisJSON.String), &analysis); err != nil {
			return questionnaires.ImportPreview{}, err
		}
		preview.Analysis = &analysis
	}
	var err error
	if preview.CreatedAt, err = parseStamp(created); err != nil {
		return questionnaires.ImportPreview{}, err
	}
	preview.UpdatedAt, err = parseStamp(updated)
	return preview, err
}
