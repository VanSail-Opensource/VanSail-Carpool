package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/questionnaires"
)

func importSettlementDataTx(ctx context.Context, tx *sql.Tx, importID string) (questionnaires.ImportPreview, []byte, rune, string, error) {
	var preview questionnaires.ImportPreview
	var raw []byte
	var delimiter, headersJSON, samplesJSON string
	var codeColumn, analysisJSON, reportJSON sql.NullString
	var created, updated string
	err := tx.QueryRowContext(ctx, `SELECT id,questionnaire_id,status,raw_csv,delimiter,headers_json,sample_rows_json,data_row_count,malformed_row_count,code_column,analysis_json,idempotency_key,report_json,created_at,updated_at
		FROM questionnaire_imports WHERE id=?`, importID).Scan(&preview.ID, &preview.QuestionnaireID, &preview.Status, &raw, &delimiter, &headersJSON, &samplesJSON,
		&preview.DataRowCount, &preview.MalformedRowCount, &codeColumn, &analysisJSON, &preview.IdempotencyKey, &reportJSON, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return questionnaires.ImportPreview{}, nil, 0, "", ErrNotFound
	}
	if err != nil {
		return questionnaires.ImportPreview{}, nil, 0, "", err
	}
	if err := json.Unmarshal([]byte(headersJSON), &preview.Headers); err != nil {
		return questionnaires.ImportPreview{}, nil, 0, "", err
	}
	if err := json.Unmarshal([]byte(samplesJSON), &preview.SampleRows); err != nil {
		return questionnaires.ImportPreview{}, nil, 0, "", err
	}
	if codeColumn.Valid {
		preview.CodeColumn = &codeColumn.String
	}
	if analysisJSON.Valid {
		var analysis questionnaires.ImportAnalysis
		if err := json.Unmarshal([]byte(analysisJSON.String), &analysis); err != nil {
			return questionnaires.ImportPreview{}, nil, 0, "", err
		}
		preview.Analysis = &analysis
	}
	preview.CreatedAt, err = parseStamp(created)
	if err != nil {
		return questionnaires.ImportPreview{}, nil, 0, "", err
	}
	preview.UpdatedAt, err = parseStamp(updated)
	if err != nil {
		return questionnaires.ImportPreview{}, nil, 0, "", err
	}
	runes := []rune(delimiter)
	if len(runes) != 1 {
		return questionnaires.ImportPreview{}, nil, 0, "", questionnaires.ErrInvalidInput
	}
	return preview, raw, runes[0], reportJSON.String, nil
}
