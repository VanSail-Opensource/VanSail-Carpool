package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// PlayLuckyDraw charges and settles an instant draw in one transaction.
func (s *Store) PlayLuckyDraw(ctx context.Context, userID, drawID, key string, rng activity.RandomSource, now time.Time) (activity.DrawResult, error) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(drawID) == "" || strings.TrimSpace(key) == "" || len(key) > 128 || rng == nil {
		return activity.DrawResult{}, activity.ErrInvalidInput
	}
	if strings.HasPrefix(key, "raffle:") {
		return activity.DrawResult{}, activity.ErrInvalidInput
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	now = now.UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.DrawResult{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if existing, loadErr := drawResultByKeyTx(ctx, tx, userID, key); loadErr == nil {
		if existing.DrawID != drawID {
			return activity.DrawResult{}, ErrConflict
		}
		existing.Replayed = true
		return existing, nil
	} else if !errors.Is(loadErr, ErrNotFound) {
		return activity.DrawResult{}, loadErr
	}
	draw, err := luckyDrawByID(ctx, tx, drawID, true)
	if err != nil {
		return activity.DrawResult{}, err
	}
	draw.Prizes, err = luckyPrizes(ctx, tx, drawID, false)
	if err != nil {
		return activity.DrawResult{}, err
	}
	if len(draw.Prizes) == 0 {
		return activity.DrawResult{}, ErrConflict
	}
	maxLoss := draw.MaximumPrizeDeduction()
	if maxLoss > math.MaxInt64-draw.FeeMinor {
		return activity.DrawResult{}, activity.ErrInvalidInput
	}
	balance, err := balanceTx(ctx, tx, userID)
	if err != nil {
		return activity.DrawResult{}, err
	}
	if balance < draw.FeeMinor+maxLoss {
		return activity.DrawResult{}, ErrInsufficientBalance
	}
	if err = eligibleDrawParticipantTx(ctx, tx, userID, draw, now); err != nil {
		return activity.DrawResult{}, err
	}
	roll, err := rng.Int63n(10000)
	if err != nil {
		return activity.DrawResult{}, err
	}
	selected := draw.Prizes[len(draw.Prizes)-1]
	for _, prize := range draw.Prizes {
		if roll < int64(prize.ProbabilityBPS) {
			selected = prize
			break
		}
		roll -= int64(prize.ProbabilityBPS)
	}
	resolved, err := resolveDrawReward(selected.Reward, rng)
	if err != nil {
		return activity.DrawResult{}, err
	}
	resultID, err := ids.New()
	if err != nil {
		return activity.DrawResult{}, err
	}
	balance, err = changeBalanceTx(ctx, tx, userID, -draw.FeeMinor, now)
	if err != nil {
		return activity.DrawResult{}, err
	}
	snapshot, err := json.Marshal(draw)
	if err != nil {
		return activity.DrawResult{}, err
	}
	rewardPayload, err := json.Marshal(resolved)
	if err != nil {
		return activity.DrawResult{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_draw_results
  (id,user_id,draw_id,prize_id,prize_name,fee_minor,reward_kind,reward_payload,balance_after_minor,
  configuration_snapshot,idempotency_key,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		resultID, userID, drawID, selected.ID, selected.Name, draw.FeeMinor, resolved.Kind, string(rewardPayload), balance, string(snapshot), key, stamp(now))
	if err != nil {
		return activity.DrawResult{}, fmt.Errorf("record instant draw: %w", err)
	}
	if _, err = insertLedgerTx(ctx, tx, userID, -draw.FeeMinor, balance, "activity_draw_fee", resultID, draw.Name, now); err != nil {
		return activity.DrawResult{}, err
	}
	balance, err = applyDrawRewardTx(ctx, tx, userID, resultID, selected.Name, resolved, balance, now)
	if err != nil {
		return activity.DrawResult{}, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_draw_results SET balance_after_minor=? WHERE id=?`, balance, resultID); err != nil {
		return activity.DrawResult{}, err
	}
	if resolved.Kind != activity.RewardNone {
		payload, _ := json.Marshal(map[string]any{"resultId": resultID})
		if err = insertOutboxTx(ctx, tx, "draw_instant_announcement", string(payload), now, now); err != nil {
			return activity.DrawResult{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return activity.DrawResult{}, err
	}
	return s.drawResultByID(ctx, resultID)
}

func eligibleDrawParticipantTx(ctx context.Context, tx *sql.Tx, userID string, draw activity.LuckyDraw, now time.Time) error {
	requiresPurchase := false
	for _, prize := range draw.Prizes {
		switch prize.Reward.Kind {
		case activity.RewardEntitlementGrant, activity.RewardSquadAccess, activity.RewardCoreComboSwitch,
			activity.RewardTrafficGrant, activity.RewardTrafficReset, activity.RewardSubscriptionExtension:
			requiresPurchase = true
		}
	}
	if !requiresPurchase {
		return nil
	}
	purchaseID, traffic, err := activeRewardPurchase(ctx, tx, userID, now)
	if err != nil {
		return err
	}
	for _, prize := range draw.Prizes {
		if prize.Reward.Kind == activity.RewardTrafficGrant && prize.Reward.Range != nil {
			minimum := prize.Reward.Range.Min
			if minimum < 0 && (minimum < math.MinInt64/(1<<30) || traffic+minimum*(1<<30) <= 0) {
				return ErrConflict
			}
			if minimum < 0 && prize.Reward.IncludeInRenewal {
				_, renewalTraffic, loadErr := drawRenewalTrafficTx(ctx, tx, purchaseID)
				if loadErr != nil {
					return loadErr
				}
				if renewalTraffic+minimum*(1<<30) <= 0 {
					return ErrConflict
				}
			}
		}
	}
	return nil
}
