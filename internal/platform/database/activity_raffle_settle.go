package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

// SettleRaffle distributes the stock in one atomic transaction or reopens after refunds.
func (s *Store) retryAttempt_SettleRaffle(ctx context.Context, id string, rng activity.RandomSource, now time.Time) (activity.RaffleSettlement, error) {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return activity.RaffleSettlement{}, err
	}
	defer func() { _ = tx.Rollback() }()
	draw, err := luckyDrawByID(ctx, tx, id, false)
	if err != nil {
		return activity.RaffleSettlement{}, err
	}
	if draw.Kind == "raffle" && (draw.Status == "completed" || draw.Status == "cancelled") {
		return activity.RaffleSettlement{Draw: draw}, nil
	}
	// A retry can arrive after an earlier attempt refunded an ineligible seat
	// and reopened the raffle, but before its outbox completion was recorded.
	if draw.Kind == "raffle" && draw.Status == "open" {
		return activity.RaffleSettlement{Draw: draw, Reopened: true}, nil
	}
	if draw.Kind != "raffle" || draw.Status != "settling" {
		return activity.RaffleSettlement{}, ErrConflict
	}
	draw.Prizes, err = luckyPrizes(ctx, tx, id, false)
	if err != nil {
		return activity.RaffleSettlement{}, err
	}
	tickets, err := activeRaffleTicketsTx(ctx, tx, id)
	if err != nil {
		return activity.RaffleSettlement{}, err
	}
	if len(tickets) != draw.Threshold {
		return activity.RaffleSettlement{}, ErrConflict
	}
	seatsByUser := make(map[string]int)
	for _, ticket := range tickets {
		seatsByUser[ticket.UserID]++
	}
	refunded := false
	for _, ticket := range tickets {
		checkErr := eligibleDrawParticipantTx(ctx, tx, ticket.UserID, draw, now)
		if checkErr == nil {
			checkErr = raffleTrafficCoverageTx(ctx, tx, ticket.UserID, draw, seatsByUser[ticket.UserID], now)
		}
		if checkErr != nil {
			if !errors.Is(checkErr, ErrConflict) {
				return activity.RaffleSettlement{}, checkErr
			}
			if err = refundRaffleTicketTx(ctx, tx, ticket, draw.Name, now); err != nil {
				return activity.RaffleSettlement{}, err
			}
			refunded = true
		}
	}
	if refunded {
		if _, err = tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET status='open',updated_at=? WHERE id=?`, stamp(now), id); err != nil {
			return activity.RaffleSettlement{}, err
		}
		payload, _ := json.Marshal(map[string]any{"drawId": id})
		if err = insertOutboxTx(ctx, tx, "draw_telegram_update", string(payload), now, now); err != nil {
			return activity.RaffleSettlement{}, err
		}
		if err = tx.Commit(); err != nil {
			return activity.RaffleSettlement{}, err
		}
		return activity.RaffleSettlement{Draw: draw, Reopened: true}, nil
	}
	slots := make([]int, 0, draw.Threshold)
	for index, prize := range draw.Prizes {
		for i := int64(0); i < prize.Stock; i++ {
			slots = append(slots, index)
		}
	}
	if len(slots) != len(tickets) {
		return activity.RaffleSettlement{}, activity.ErrInvalidInput
	}
	// Fisher-Yates with the project's cryptographic integer source.
	for i := len(slots) - 1; i > 0; i-- {
		j, rollErr := rng.Int63n(int64(i + 1))
		if rollErr != nil {
			return activity.RaffleSettlement{}, rollErr
		}
		slots[i], slots[j] = slots[j], slots[i]
	}
	result := activity.RaffleSettlement{Draw: draw, Results: make([]activity.DrawResult, 0, len(tickets))}
	winners := make(map[string]bool)
	all := make(map[string]bool)
	snapshot, snapshotErr := json.Marshal(draw)
	if snapshotErr != nil {
		return activity.RaffleSettlement{}, snapshotErr
	}
	for index, ticket := range tickets {
		prize := draw.Prizes[slots[index]]
		resolved, rollErr := resolveDrawReward(prize.Reward, rng)
		if rollErr != nil {
			return activity.RaffleSettlement{}, rollErr
		}
		resultID, idErr := ids.New()
		if idErr != nil {
			return activity.RaffleSettlement{}, idErr
		}
		balance, balanceErr := balanceTx(ctx, tx, ticket.UserID)
		if balanceErr != nil {
			return activity.RaffleSettlement{}, balanceErr
		}
		if ticket.Reserve > 0 {
			balance, balanceErr = changeBalanceTx(ctx, tx, ticket.UserID, ticket.Reserve, now)
			if balanceErr != nil {
				return activity.RaffleSettlement{}, balanceErr
			}
			if _, err = insertLedgerTx(ctx, tx, ticket.UserID, ticket.Reserve, balance, "activity_raffle_reserve_release", ticket.ID, draw.Name, now); err != nil {
				return activity.RaffleSettlement{}, err
			}
		}
		payload, marshalErr := json.Marshal(resolved)
		if marshalErr != nil {
			return activity.RaffleSettlement{}, marshalErr
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO activity_draw_results
   (id,user_id,draw_id,prize_id,prize_name,fee_minor,reward_kind,reward_payload,balance_after_minor,
   configuration_snapshot,idempotency_key,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			resultID, ticket.UserID, id, prize.ID, prize.Name, ticket.Fee, resolved.Kind, string(payload), balance, string(snapshot), "raffle:"+ticket.ID, stamp(now))
		if err != nil {
			return activity.RaffleSettlement{}, fmt.Errorf("record raffle reward: %w", err)
		}
		balance, err = applyDrawRewardTx(ctx, tx, ticket.UserID, resultID, prize.Name, resolved, balance, now)
		if err != nil {
			return activity.RaffleSettlement{}, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE activity_draw_results SET balance_after_minor=? WHERE id=?`, balance, resultID); err != nil {
			return activity.RaffleSettlement{}, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE activity_raffle_tickets SET status='settled',prize_id=?,result_id=?,updated_at=? WHERE id=?`,
			prize.ID, resultID, stamp(now), ticket.ID); err != nil {
			return activity.RaffleSettlement{}, err
		}
		all[ticket.UserID] = true
		if resolved.Kind != activity.RewardNone {
			winners[ticket.UserID] = true
		}
		result.Results = append(result.Results, activity.DrawResult{ID: resultID, UserID: ticket.UserID, DrawID: id, PrizeID: prize.ID,
			PrizeName: prize.Name, FeeMinor: ticket.Fee, Reward: resolved, BalanceAfterMinor: balance, CreatedAt: now})
		if resolved.Kind != activity.RewardNone {
			notification, _ := json.Marshal(map[string]any{"resultId": resultID})
			if err = insertOutboxTx(ctx, tx, "draw_raffle_private", string(notification), now, now); err != nil {
				return activity.RaffleSettlement{}, err
			}
		}
	}
	for userID := range all {
		if !winners[userID] {
			result.NonWinners = append(result.NonWinners, userID)
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET status='completed',updated_at=? WHERE id=?`, stamp(now), id); err != nil {
		return activity.RaffleSettlement{}, err
	}
	payload, _ := json.Marshal(map[string]any{"drawId": id})
	if err = insertOutboxTx(ctx, tx, "draw_raffle_complete", string(payload), now, now); err != nil {
		return activity.RaffleSettlement{}, err
	}
	if err = tx.Commit(); err != nil {
		return activity.RaffleSettlement{}, err
	}
	return result, nil
}
