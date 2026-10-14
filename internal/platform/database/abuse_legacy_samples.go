package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func legacySamplesTx(ctx context.Context, tx *sql.Tx, cutoff time.Time, limit int) ([]abuse.Sample, error) {
	query := `SELECT user_id,node_uuid,bucket_at,reason_name,qps_limit,qps FROM abuse_qps_samples WHERE bucket_at<=? ORDER BY bucket_at,user_id,reason_name,node_uuid`
	args := []any{stamp(cutoff)}
	var boundaryAt, boundaryUser string
	err := tx.QueryRowContext(ctx, `SELECT bucket_at,user_id FROM abuse_qps_samples WHERE bucket_at<=? ORDER BY bucket_at,user_id,reason_name,node_uuid LIMIT 1 OFFSET ?`, stamp(cutoff), limit-1).Scan(&boundaryAt, &boundaryUser)
	if err == nil {
		query = `SELECT user_id,node_uuid,bucket_at,reason_name,qps_limit,qps FROM abuse_qps_samples WHERE bucket_at<=? AND (bucket_at<? OR (bucket_at=? AND user_id<=?)) ORDER BY bucket_at,user_id,reason_name,node_uuid`
		args = []any{stamp(cutoff), boundaryAt, boundaryAt, boundaryUser}
	} else if err != sql.ErrNoRows {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []abuse.Sample{}
	for rows.Next() {
		var item abuse.Sample
		var at string
		if err = rows.Scan(&item.UserID, &item.NodeUUID, &at, &item.ReasonName, &item.QPSLimit, &item.Count); err != nil {
			return nil, err
		}
		item.BucketAt, err = parseStamp(at)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func deleteLegacySamplesTx(ctx context.Context, tx *sql.Tx, samples []abuse.Sample) error {
	for _, item := range samples {
		_, err := tx.ExecContext(ctx, `DELETE FROM abuse_qps_samples WHERE user_id=? AND node_uuid=? AND bucket_at=? AND reason_name=?`, item.UserID, item.NodeUUID, stamp(item.BucketAt), item.ReasonName)
		if err != nil {
			return err
		}
	}
	return nil
}
