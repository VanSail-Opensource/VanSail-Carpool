package database

import (
	"context"
	"encoding/json"
	"strconv"
	"time"
)

// TelegramStarRefundReview preserves authenticated outgoing proof which cannot
// be applied automatically, without deciding a new refund or affiliate policy.
type TelegramStarRefundReview struct {
	BotID         int64  `json:"botId"`
	OrderID       string `json:"orderId"`
	ChargeID      string `json:"chargeId"`
	PayableAmount string `json:"payableAmount"`
	Currency      string `json:"currency"`
	TelegramID    int64  `json:"telegramId"`
	ProofHash     string `json:"proofHash"`
	Reason        string `json:"reason"`
}

// RecordTelegramStarRefundReview is idempotent across page retries and rewind.
// The existing immutable audit surface makes unresolved monetary proof visible.
func (s *Store) RecordTelegramStarRefundReview(ctx context.Context, proof TelegramStarRefundReview, now time.Time) error {
	if proof.BotID <= 0 || proof.OrderID == "" || proof.ChargeID == "" || proof.TelegramID <= 0 ||
		proof.PayableAmount == "" || proof.Currency != "XTR" || !validTelegramStarAnchor(1, proof.ProofHash) || proof.Reason == "" {
		return ErrConflict
	}
	detail, err := json.Marshal(proof)
	if err != nil {
		return err
	}
	id := "stars-refund-review:" + strconv.FormatInt(proof.BotID, 10) + ":" + proof.ProofHash
	_, err = s.db.ExecContext(ctx, `INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
		VALUES($1,NULL,'billing.stars.refund_review_required','payment_order',$2,$3,$4) ON CONFLICT(id) DO NOTHING`, id, proof.OrderID, string(detail), stamp(now))
	return err
}
