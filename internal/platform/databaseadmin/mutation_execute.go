package databaseadmin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"strings"
	"time"
)

func (s *Service) execute(ctx context.Context, tx *sql.Tx, action string, prepared preparedMutation) (MutationResult, error) {
	schema := prepared.schema
	switch action {
	case "insert":
		query := `INSERT INTO ` + quoteIdentifier(schema.Name)
		if len(prepared.columns) == 0 {
			query += ` DEFAULT VALUES`
		} else {
			names := make([]string, 0, len(prepared.columns))
			for _, column := range prepared.columns {
				names = append(names, quoteIdentifier(column.Name))
			}
			query += ` (` + strings.Join(names, `,`) + `) VALUES (` + placeholders(len(names)) + `)`
		}
		query += ` RETURNING ` + selectList(schema)
		data, err := s.scanRecord(schema, tx.QueryRowContext(ctx, query, prepared.arguments...))
		if err != nil {
			return MutationResult{}, fmt.Errorf("insert %s record: %w", schema.Name, err)
		}
		return MutationResult{Row: &data.record}, nil
	case "update":
		sets := make([]string, 0, len(prepared.columns))
		for _, column := range prepared.columns {
			sets = append(sets, quoteIdentifier(column.Name)+`=?`)
		}
		query := `UPDATE ` + quoteIdentifier(schema.Name) + ` SET ` + strings.Join(sets, `,`) + ` WHERE ` + keyTuple(schema) + `=(` + placeholders(len(prepared.current.keyRaw)) + `) RETURNING ` + selectList(schema)
		args := append(append([]any(nil), prepared.arguments...), prepared.current.keyRaw...)
		data, err := s.scanRecord(schema, tx.QueryRowContext(ctx, query, args...))
		if err != nil {
			return MutationResult{}, fmt.Errorf("update %s record: %w", schema.Name, err)
		}
		return MutationResult{Row: &data.record}, nil
	case "delete":
		query := `DELETE FROM ` + quoteIdentifier(schema.Name) + ` WHERE ` + keyTuple(schema) + `=(` + placeholders(len(prepared.current.keyRaw)) + `)`
		result, err := tx.ExecContext(ctx, query, prepared.current.keyRaw...)
		if err != nil {
			return MutationResult{}, fmt.Errorf("delete %s record: %w", schema.Name, err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return MutationResult{}, ErrOptimisticConflict
		}
		return MutationResult{Deleted: true}, nil
	default:
		return MutationResult{}, ErrInvalidValue
	}
}

func (s *Service) auditMutation(ctx context.Context, tx *sql.Tx, actor string, request MutationRequest, prepared preparedMutation, result MutationResult, rescueBackupID string) error {
	auditID, err := ids.New()
	if err != nil {
		return fmt.Errorf("create database edit audit identifier: %w", err)
	}
	detail := map[string]any{
		"action": request.Action, "table": request.Table, "key": request.Key,
		"changedColumns": prepared.changedColumns, "reason": request.Reason, "rescueBackupId": rescueBackupID,
		"beforeHash": "", "afterHash": "", "values": "[redacted]", "warning": bypassWarning,
	}
	if prepared.current != nil {
		detail["beforeHash"] = prepared.current.record.RecordHash
	}
	if result.Row != nil {
		detail["afterHash"] = result.Row.RecordHash
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("encode database edit audit: %w", err)
	}
	targetID := request.Table
	if len(request.Key) > 0 {
		keyData, _ := json.Marshal(request.Key)
		targetID = request.Table + ":" + string(keyData)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at) VALUES(?,?,?,?,?,?,?)`,
		auditID, actor, "database_record_"+request.Action, "database_table", targetID, string(encoded), s.now().UTC().Format(time.RFC3339Nano)); err != nil {
		return fmt.Errorf("record database edit audit: %w", err)
	}
	return nil
}

func cloneKey(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneValues(source map[string]Value) map[string]Value {
	if source == nil {
		return nil
	}
	result := make(map[string]Value, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
