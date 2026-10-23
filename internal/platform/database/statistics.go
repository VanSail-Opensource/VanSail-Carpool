package database

import (
	"context"
	"database/sql"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"time"
)

type statisticEvent struct {
	userID     string
	input      int64
	output     int64
	discount   int64
	addon      int64
	won        bool
	outcomeSet bool
	created    time.Time
}

// ComboStatistics reports server-charged purchase facts while squad membership
// remains live through the referenced combo record.

func (s *Store) ComboStatistics(ctx context.Context, comboID string, from, to time.Time, bucket string, location *time.Location) (model.AdminStatistics, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.user_id,p.charged_txb_minor,p.coupon_discount_txb_minor,
		COALESCE((SELECT SUM(charged_txb_minor) FROM purchase_addons WHERE purchase_id=p.id),0),p.created_at
		FROM purchases p WHERE p.combo_id=? AND p.status NOT IN ('cancelled','failed') AND p.created_at>=? AND p.created_at<? ORDER BY p.created_at`,
		comboID, stamp(from), stamp(to))
	if err != nil {
		return model.AdminStatistics{}, err
	}
	events, err := scanStatisticEvents(rows, func(rows *sql.Rows) (statisticEvent, error) {
		var event statisticEvent
		var created string
		err := rows.Scan(&event.userID, &event.input, &event.discount, &event.addon, &created)
		if err == nil {
			event.created, err = parseStamp(created)
		}
		return event, err
	})
	if err != nil {
		return model.AdminStatistics{}, err
	}
	distribution, err := s.catalogSquadDistribution(ctx, comboID, "", from, to)
	if err != nil {
		return model.AdminStatistics{}, err
	}
	return aggregateStatistics(comboID, from, to, bucket, location, events, distribution), nil
}

// SquadStatistics reports included-versus-addon selection usage for one live
// Proxy internal squad UUID.

func (s *Store) SquadStatistics(ctx context.Context, squadUUID string, from, to time.Time, bucket string, location *time.Location) (model.AdminStatistics, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.user_id,p.charged_txb_minor,p.coupon_discount_txb_minor,
		COALESCE((SELECT SUM(charged_txb_minor) FROM purchase_addons WHERE purchase_id=p.id AND group_id=?),0),p.created_at
		FROM purchases p JOIN combos c ON c.id=p.combo_id
		WHERE p.status NOT IN ('cancelled','failed') AND p.created_at>=? AND p.created_at<? AND
		(EXISTS (SELECT 1 FROM json_each(c.included_squad_uuids) WHERE value=?) OR EXISTS (SELECT 1 FROM purchase_addons WHERE purchase_id=p.id AND group_id=?))
		ORDER BY p.created_at`, squadUUID, stamp(from), stamp(to), squadUUID, squadUUID)
	if err != nil {
		return model.AdminStatistics{}, err
	}
	events, err := scanStatisticEvents(rows, func(rows *sql.Rows) (statisticEvent, error) {
		var event statisticEvent
		var created string
		err := rows.Scan(&event.userID, &event.input, &event.discount, &event.addon, &created)
		if err == nil {
			event.created, err = parseStamp(created)
		}
		return event, err
	})
	if err != nil {
		return model.AdminStatistics{}, err
	}
	distribution, err := s.catalogSquadDistribution(ctx, "", squadUUID, from, to)
	if err != nil {
		return model.AdminStatistics{}, err
	}
	return aggregateStatistics(squadUUID, from, to, bucket, location, events, distribution), nil
}

func (s *Store) catalogSquadDistribution(ctx context.Context, comboID, squadUUID string, from, to time.Time) ([]model.StatisticSlice, error) {
	query := `SELECT kind,uuid,COUNT(*) FROM (
		SELECT 'included' AS kind,j.value AS uuid FROM purchases p JOIN combos c ON c.id=p.combo_id, json_each(c.included_squad_uuids) j
		WHERE p.status NOT IN ('cancelled','failed') AND p.created_at>=? AND p.created_at<?
		UNION ALL
		SELECT 'addon',a.group_id FROM purchases p JOIN purchase_addons a ON a.purchase_id=p.id
		WHERE p.status NOT IN ('cancelled','failed') AND p.created_at>=? AND p.created_at<?) usage WHERE 1=1`
	args := []any{stamp(from), stamp(to), stamp(from), stamp(to)}
	if comboID != "" {
		query = `SELECT kind,uuid,COUNT(*) FROM (
			SELECT 'included' AS kind,j.value AS uuid FROM purchases p JOIN combos c ON c.id=p.combo_id, json_each(c.included_squad_uuids) j
			WHERE p.combo_id=? AND p.status NOT IN ('cancelled','failed') AND p.created_at>=? AND p.created_at<?
			UNION ALL SELECT 'addon',a.group_id FROM purchases p JOIN purchase_addons a ON a.purchase_id=p.id
			WHERE p.combo_id=? AND p.status NOT IN ('cancelled','failed') AND p.created_at>=? AND p.created_at<?) usage WHERE 1=1`
		args = []any{comboID, stamp(from), stamp(to), comboID, stamp(from), stamp(to)}
	}
	if squadUUID != "" {
		query += ` AND uuid=?`
		args = append(args, squadUUID)
	}
	query += ` GROUP BY kind,uuid ORDER BY kind,uuid`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]model.StatisticSlice, 0)
	for rows.Next() {
		var kind, uuid string
		var count int64
		if err := rows.Scan(&kind, &uuid, &count); err != nil {
			return nil, err
		}
		result = append(result, model.StatisticSlice{ID: kind + ":" + uuid, Label: kind, Count: count})
	}
	return result, rows.Err()
}

func (s *Store) ActivityGameStatistics(ctx context.Context, gameID string, from, to time.Time, bucket string, location *time.Location) (model.AdminStatistics, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id,stake_minor,payout_minor,won,created_at FROM activity_bets WHERE game_id=? AND created_at>=? AND created_at<? ORDER BY created_at`, gameID, stamp(from), stamp(to))
	if err != nil {
		return model.AdminStatistics{}, err
	}
	events, err := scanStatisticEvents(rows, func(rows *sql.Rows) (statisticEvent, error) {
		var event statisticEvent
		var won int
		var created string
		err := rows.Scan(&event.userID, &event.input, &event.output, &won, &created)
		event.won = won == 1
		event.outcomeSet = true
		if err == nil {
			event.created, err = parseStamp(created)
		}
		return event, err
	})
	if err != nil {
		return model.AdminStatistics{}, err
	}
	stats := aggregateStatistics(gameID, from, to, bucket, location, events, nil)
	stats.Distribution = []model.StatisticSlice{{ID: "win", Label: "win", Count: stats.Wins}, {ID: "loss", Label: "loss", Count: stats.Losses}}
	return stats, nil
}
