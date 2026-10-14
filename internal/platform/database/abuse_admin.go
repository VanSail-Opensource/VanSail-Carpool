package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func (s *Store) NodeCredentials(ctx context.Context) ([]abuse.NodeCredential, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_uuid,node_name,last_report_at,rotated_at FROM abuse_node_credentials ORDER BY node_name,node_uuid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []abuse.NodeCredential{}
	for rows.Next() {
		var item abuse.NodeCredential
		var reported sql.NullString
		var rotated string
		if err = rows.Scan(&item.UUID, &item.Name, &reported, &rotated); err != nil {
			return nil, err
		}
		if reported.Valid {
			parsed, parseErr := parseStamp(reported.String)
			if parseErr != nil {
				return nil, parseErr
			}
			item.LastReportAt = &parsed
		}
		parsed, parseErr := parseStamp(rotated)
		if parseErr != nil {
			return nil, parseErr
		}
		item.RotatedAt = parsed
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) SaveNodeCredential(ctx context.Context, node abuse.Node, digest, sealed string, now time.Time) error {
	if node.UUID == "" || digest == "" || sealed == "" {
		return abuse.ErrInvalid
	}
	// Rotation must update an existing row; it cannot resurrect a key removed
	// concurrently by a complete upstream inventory reconciliation.
	if node.Name == "" {
		result, err := s.db.ExecContext(ctx, `UPDATE abuse_node_credentials SET key_digest=?,sealed_key=?,rotated_at=?,updated_at=? WHERE node_uuid=?`, digest, sealed, stamp(now), stamp(now), node.UUID)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrNotFound
		}
		return nil
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO abuse_node_credentials(node_uuid,node_name,key_digest,sealed_key,rotated_at,created_at,updated_at) VALUES(?,?,?,?,?,?,?) ON CONFLICT(node_uuid) DO UPDATE SET node_name=CASE WHEN excluded.node_name='' THEN abuse_node_credentials.node_name ELSE excluded.node_name END,key_digest=excluded.key_digest,sealed_key=excluded.sealed_key,rotated_at=excluded.rotated_at,updated_at=excluded.updated_at`, node.UUID, node.Name, digest, sealed, stamp(now), stamp(now), stamp(now))
	return err
}
func (s *Store) CopyNodeCredential(ctx context.Context, nodeID string) (string, error) {
	var sealed string
	err := s.db.QueryRowContext(ctx, `SELECT sealed_key FROM abuse_node_credentials WHERE node_uuid=?`, nodeID).Scan(&sealed)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return sealed, err
}
func (s *Store) Statistics(ctx context.Context, now time.Time) (map[string]float64, error) {
	var avg, min, max sql.NullFloat64
	err := s.db.QueryRowContext(ctx, `SELECT CAST(SUM(qps_sum) AS REAL)/NULLIF(SUM(observation_count),0),MIN(qps_min),MAX(qps_max) FROM abuse_qps_rollups WHERE window_at>=?`, stamp(now.Add(-24*time.Hour))).Scan(&avg, &min, &max)
	if err != nil {
		return nil, fmt.Errorf("abuse statistics: %w", err)
	}
	return map[string]float64{"average": avg.Float64, "minimum": min.Float64, "maximum": max.Float64}, nil
}
