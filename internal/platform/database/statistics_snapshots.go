package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	productstats "github.com/VanSail-Opensource/VanSail-Carpool/internal/statistics"
)

// SaveStatisticsPartition atomically replaces one independent last-good JSON part.
func (s *Store) SaveStatisticsPartition(ctx context.Context, partition string, payload []byte, generatedAt time.Time) error {
	partition = strings.TrimSpace(partition)
	var object map[string]any
	if partition == "" || len(partition) > 80 || len(payload) == 0 || len(payload) > 2*1024*1024 ||
		json.Unmarshal(payload, &object) != nil || object == nil {
		return fmt.Errorf("save statistics partition: invalid partition payload")
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return fmt.Errorf("canonicalize statistics partition: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO statistics_snapshots(partition,payload_json,generated_at)
		VALUES(?,?,?) ON CONFLICT(partition) DO UPDATE SET payload_json=excluded.payload_json,
		generated_at=excluded.generated_at`, partition, string(canonical), stamp(generatedAt))
	if err != nil {
		return fmt.Errorf("save statistics partition: %w", err)
	}
	return nil
}

// LoadStatisticsPartition returns one independent last-good JSON part.
func (s *Store) LoadStatisticsPartition(ctx context.Context, partition string) ([]byte, time.Time, error) {
	var payload, generated string
	err := s.db.QueryRowContext(ctx, `SELECT payload_json,generated_at FROM statistics_snapshots WHERE partition=?`,
		strings.TrimSpace(partition)).Scan(&payload, &generated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, time.Time{}, productstats.ErrPartitionNotFound
		}
		return nil, time.Time{}, fmt.Errorf("load statistics partition: %w", err)
	}
	generatedAt, err := parseStamp(generated)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("parse statistics partition timestamp: %w", err)
	}
	return []byte(payload), generatedAt, nil
}
