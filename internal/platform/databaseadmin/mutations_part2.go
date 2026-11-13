package databaseadmin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

func mutationDigest(actor string, request MutationRequest) (string, error) {
	request.ReviewHash = ""
	request.Confirmation = ""
	payload := struct {
		Actor   string          `json:"actor"`
		Request MutationRequest `json:"request"`
	}{Actor: actor, Request: request}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode database mutation review: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (s *Service) checkReview(ctx context.Context, actor string, request MutationRequest, digest string) error {
	if err := requireMutationActor(ctx, s.db, actor); err != nil {
		return err
	}
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM database_admin_reviews
		WHERE id=$1 AND actor_user_id=$2 AND action=$3 AND table_name=$4 AND request_hash=$5 AND consumed_at IS NULL AND expires_at::timestamptz>$6::timestamptz`,
		request.ReviewHash, actor, request.Action, request.Table, digest, s.now().UTC().Format(time.RFC3339Nano)).Scan(&count)
	if err != nil {
		return fmt.Errorf("verify database review: %w", err)
	}
	if count != 1 {
		return ErrReviewConflict
	}
	return nil
}

func consumeReview(ctx context.Context, tx *sql.Tx, actor string, request MutationRequest, digest string, now time.Time) error {
	result, err := tx.ExecContext(ctx, `UPDATE database_admin_reviews SET consumed_at=$1
		WHERE id=$2 AND actor_user_id=$3 AND action=$4 AND table_name=$5 AND request_hash=$6 AND consumed_at IS NULL AND expires_at::timestamptz>$7::timestamptz`,
		now.Format(time.RFC3339Nano), request.ReviewHash, actor, request.Action, request.Table, digest, now.Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("consume database review: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("count consumed database review: %w", err)
	}
	if affected != 1 {
		return ErrReviewConflict
	}
	return nil
}
