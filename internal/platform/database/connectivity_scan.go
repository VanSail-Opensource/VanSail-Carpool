package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

const connectivityAttemptColumns = `id,run_id,host_uuid,config_hash,remnawave_user_id,trigger,
	started_at,finished_at,status,latency_ms,http_status,error_code`

func scanConnectivityAttempt(row rowScanner) (connectivity.Attempt, error) {
	var attempt connectivity.Attempt
	var host, finished sql.NullString
	var started string
	var latency sql.NullFloat64
	var httpStatus sql.NullInt64
	if err := row.Scan(&attempt.ID, &attempt.RunID, &host, &attempt.ConfigHash, &attempt.RemnawaveUserID,
		&attempt.Trigger, &started, &finished, &attempt.Status, &latency, &httpStatus, &attempt.ErrorCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return connectivity.Attempt{}, ErrNotFound
		}
		return connectivity.Attempt{}, fmt.Errorf("scan connectivity attempt: %w", err)
	}
	attempt.HostUUID = host.String
	var err error
	attempt.StartedAt, err = parseStamp(started)
	if err != nil {
		return connectivity.Attempt{}, fmt.Errorf("parse connectivity start timestamp: %w", err)
	}
	attempt.FinishedAt, err = parseOptionalStamp(finished)
	if err != nil {
		return connectivity.Attempt{}, fmt.Errorf("parse connectivity finish timestamp: %w", err)
	}
	if latency.Valid {
		attempt.LatencyMS = &latency.Float64
	}
	if httpStatus.Valid {
		value := int(httpStatus.Int64)
		attempt.HTTPStatus = &value
	}
	return attempt, nil
}

func collectConnectivityAttempts(rows *sql.Rows, capacity int) ([]connectivity.Attempt, error) {
	defer func() { _ = rows.Close() }()
	items := make([]connectivity.Attempt, 0, capacity)
	for rows.Next() {
		attempt, err := scanConnectivityAttempt(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read connectivity attempts: %w", err)
	}
	return items, nil
}
