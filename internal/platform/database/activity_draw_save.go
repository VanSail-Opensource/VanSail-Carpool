package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func saveLuckyDrawTx(ctx context.Context, tx *sql.Tx, input activity.LuckyDrawInput, now time.Time) (activity.LuckyDraw, error) {
	var err error
	input.Prizes = append([]activity.PrizeInput(nil), input.Prizes...)
	input.Name, input.Description = strings.TrimSpace(input.Name), strings.TrimSpace(input.Description)
	input.Keyword, input.Command = strings.TrimSpace(input.Keyword), strings.TrimSpace(input.Command)
	activeSeats := 0
	previousFee := int64(0)
	if input.ID == "" {
		input.Status = "draft"
		if input.Kind == "instant" && input.Enabled {
			input.Status = "open"
		}
		input.Revision = 1
		input.ID, err = ids.New()
		if err != nil {
			return activity.LuckyDraw{}, err
		}
	} else {
		previous, loadErr := lockedLuckyDrawTx(ctx, tx, input.ID, false)
		if loadErr != nil {
			return activity.LuckyDraw{}, loadErr
		}
		if previous.Kind != input.Kind || previous.Status == "completed" || previous.Status == "cancelled" ||
			(input.Kind == "raffle" && previous.Status != "draft" && previous.Status != "open") {
			return activity.LuckyDraw{}, ErrConflict
		}
		input.Status = previous.Status
		previousFee = previous.FeeMinor
		if input.Kind == "instant" {
			input.Status = "draft"
			if input.Enabled {
				input.Status = "open"
			}
		}
		input.GroupChatID, input.AnnouncementMessageID = previous.GroupChatID, previous.AnnouncementMessageID
		input.Revision = previous.Revision + 1
		if input.Kind == "raffle" {
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_raffle_tickets WHERE draw_id=$1 AND status='active'`, input.ID).Scan(&activeSeats); err != nil {
				return activity.LuckyDraw{}, err
			}
			if input.Threshold < activeSeats {
				return activity.LuckyDraw{}, ErrConflict
			}
		}
	}
	if err := input.Validate(); err != nil {
		return activity.LuckyDraw{}, err
	}
	if input.Kind == "raffle" && input.Status == "open" {
		if err := requireRaffleTriggersAvailableTx(ctx, tx, input); err != nil {
			return activity.LuckyDraw{}, err
		}
		if err := validateRaffleTrafficForEntriesTx(ctx, tx, activity.LuckyDraw{LuckyDrawInput: input}, now); err != nil {
			return activity.LuckyDraw{}, err
		}
		if err := reconcileRaffleReservationsTx(ctx, tx, input.ID, input.MaximumPrizeDeduction(), input.Name, input.Revision, now); err != nil {
			return activity.LuckyDraw{}, err
		}
	}
	if input.Revision == 1 {
		_, err = tx.ExecContext(ctx, `INSERT INTO activity_lucky_draws
   (id,name,description,kind,status,fee_minor,expected_participation,threshold,keyword,command,revision,created_at,updated_at)
   VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, input.ID, input.Name, input.Description, input.Kind, input.Status, input.FeeMinor,
			nullablePositiveInt(input.ExpectedParticipation), nullablePositiveInt(input.Threshold), input.Keyword, input.Command, input.Revision, stamp(now), stamp(now))
	} else {
		result, updateErr := tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET name=$1,description=$2,status=$3,fee_minor=$4,
   expected_participation=$5,threshold=$6,keyword=$7,command=$8,revision=$9,updated_at=$10 WHERE id=$11`,
			input.Name, input.Description, input.Status, input.FeeMinor, nullablePositiveInt(input.ExpectedParticipation),
			nullablePositiveInt(input.Threshold), input.Keyword, input.Command, input.Revision, stamp(now), input.ID)
		err = updateErr
		if err == nil {
			affected, rowsErr := result.RowsAffected()
			if rowsErr != nil {
				return activity.LuckyDraw{}, rowsErr
			}
			if affected != 1 {
				return activity.LuckyDraw{}, ErrNotFound
			}
		}
	}
	if err != nil {
		if isUniqueConstraint(err) {
			return activity.LuckyDraw{}, ErrConflict
		}
		return activity.LuckyDraw{}, fmt.Errorf("save lucky draw: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM activity_lucky_prizes WHERE draw_id=$1`, input.ID); err != nil {
		return activity.LuckyDraw{}, err
	}
	for position, prize := range input.Prizes {
		prizeID := prize.ID
		if prizeID == "" {
			prizeID, err = ids.New()
			if err != nil {
				return activity.LuckyDraw{}, err
			}
		}
		input.Prizes[position].ID = prizeID
		input.Prizes[position].Name = strings.TrimSpace(prize.Name)
		payload, marshalErr := json.Marshal(prize.Reward)
		if marshalErr != nil {
			return activity.LuckyDraw{}, marshalErr
		}
		var probability, stock any
		if input.Kind == "instant" {
			probability = prize.ProbabilityBPS
		} else {
			stock = prize.Stock
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_lucky_prizes(id,draw_id,name,position,probability_bps,stock,reward_payload)
			VALUES($1,$2,$3,$4,$5,$6,$7)`, prizeID, input.ID, strings.TrimSpace(prize.Name), position, probability, stock, string(payload)); err != nil {
			if isUniqueConstraint(err) {
				return activity.LuckyDraw{}, ErrConflict
			}
			return activity.LuckyDraw{}, fmt.Errorf("save lucky-draw prize: %w", err)
		}
	}
	snapshot, err := json.Marshal(input)
	if err != nil {
		return activity.LuckyDraw{}, err
	}
	revisionID, err := ids.New()
	if err != nil {
		return activity.LuckyDraw{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_draw_revisions(id,draw_id,revision,snapshot_json,created_at) VALUES($1,$2,$3,$4,$5)`,
		revisionID, input.ID, input.Revision, string(snapshot), stamp(now)); err != nil {
		return activity.LuckyDraw{}, err
	}
	if input.Kind == "raffle" && input.Status == "open" {
		payload, _ := json.Marshal(map[string]any{"drawId": input.ID, "revision": input.Revision})
		if err := insertOutboxTx(ctx, tx, "draw_telegram_update", string(payload), now, now); err != nil {
			return activity.LuckyDraw{}, err
		}
		if previousFee > 0 && previousFee != input.FeeMinor {
			notice, _ := json.Marshal(map[string]any{"drawId": input.ID, "revision": input.Revision,
				"previousFeeMinor": previousFee, "newFeeMinor": input.FeeMinor})
			if err := insertOutboxTx(ctx, tx, "draw_raffle_price_notice", string(notice), now, now); err != nil {
				return activity.LuckyDraw{}, err
			}
		}
		if activeSeats == input.Threshold {
			if _, err := tx.ExecContext(ctx, `UPDATE activity_lucky_draws SET status='settling',updated_at=$1 WHERE id=$2`, stamp(now), input.ID); err != nil {
				return activity.LuckyDraw{}, err
			}
			if err := insertOutboxTx(ctx, tx, "draw_raffle_settle", string(payload), now, now); err != nil {
				return activity.LuckyDraw{}, err
			}
		}
	}
	draw, err := luckyDrawByID(ctx, tx, input.ID, false)
	if err != nil {
		return activity.LuckyDraw{}, err
	}
	draw.Prizes, err = luckyPrizes(ctx, tx, input.ID)
	if err == nil && draw.Kind == "raffle" {
		err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_raffle_tickets WHERE draw_id=$1 AND status IN ('active','settled')`, draw.ID).Scan(&draw.Seats)
	}
	return draw, err
}
