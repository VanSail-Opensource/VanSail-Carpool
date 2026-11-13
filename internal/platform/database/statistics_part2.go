package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"math"
	"sort"
	"time"
)

func (s *Store) LuckyDrawStatistics(ctx context.Context, drawID string, from, to time.Time, bucket string, location *time.Location) (model.AdminStatistics, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return model.AdminStatistics{}, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT result.user_id,result.fee_minor,COALESCE(reward.delta_txb_minor,0),result.created_at
		FROM activity_draw_results result LEFT JOIN ledger_entries reward ON reward.kind='activity_draw_reward'
		AND reward.reference_id=result.id AND reward.user_id=result.user_id
		WHERE result.draw_id=$1 AND `+statisticTimeRangeSQL("result.created_at", "$2", "$3")+` ORDER BY result.created_at`, drawID, purchaseTimestampOrder(from), purchaseTimestampOrder(to))
	if err != nil {
		return model.AdminStatistics{}, err
	}
	events, err := scanStatisticEvents(rows, func(rows *sql.Rows) (statisticEvent, error) {
		var event statisticEvent
		var created string
		err := rows.Scan(&event.userID, &event.input, &event.output, &created)
		if err == nil {
			event.created, err = parseStamp(created)
		}
		return event, err
	})
	if err != nil {
		return model.AdminStatistics{}, err
	}
	distributionRows, err := tx.QueryContext(ctx, `SELECT prize_id,prize_name,COUNT(*) FROM activity_draw_results WHERE draw_id=$1 AND `+statisticTimeRangeSQL("created_at", "$2", "$3")+` GROUP BY prize_id,prize_name ORDER BY COUNT(*) DESC,prize_name`, drawID, purchaseTimestampOrder(from), purchaseTimestampOrder(to))
	if err != nil {
		return model.AdminStatistics{}, err
	}
	defer func() { _ = distributionRows.Close() }()
	distribution := make([]model.StatisticSlice, 0)
	for distributionRows.Next() {
		var item model.StatisticSlice
		if err := distributionRows.Scan(&item.ID, &item.Label, &item.Count); err != nil {
			return model.AdminStatistics{}, err
		}
		distribution = append(distribution, item)
	}
	if err := distributionRows.Err(); err != nil {
		return model.AdminStatistics{}, err
	}
	if err = tx.Commit(); err != nil {
		return model.AdminStatistics{}, err
	}
	return aggregateStatistics(drawID, from, to, bucket, location, events, distribution)
}

func scanStatisticEvents(rows *sql.Rows, scan func(*sql.Rows) (statisticEvent, error)) ([]statisticEvent, error) {
	defer func() { _ = rows.Close() }()
	result := make([]statisticEvent, 0)
	for rows.Next() {
		event, err := scan(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func aggregateStatistics(resourceID string, from, to time.Time, bucket string, location *time.Location, events []statisticEvent, distribution []model.StatisticSlice) (model.AdminStatistics, error) {
	if location == nil {
		location = time.UTC
	}
	stats := model.AdminStatistics{ResourceID: resourceID, TimeZone: location.String(), From: from.In(location).Format(time.DateOnly), To: to.Add(-time.Nanosecond).In(location).Format(time.DateOnly), Bucket: bucket, Series: []model.StatisticPoint{}, Distribution: distribution}
	users := make(map[string]struct{})
	type bucketAggregate struct {
		point model.StatisticPoint
		users map[string]struct{}
	}
	buckets := make(map[string]*bucketAggregate)
	for _, event := range events {
		stats.Count++
		users[event.userID] = struct{}{}
		if event.outcomeSet {
			if event.won {
				stats.Wins++
			} else {
				stats.Losses++
			}
		}
		key := statisticBucket(event.created.In(location), bucket)
		aggregate := buckets[key]
		if aggregate == nil {
			aggregate = &bucketAggregate{point: model.StatisticPoint{PeriodStart: key}, users: make(map[string]struct{})}
			buckets[key] = aggregate
		}
		aggregate.point.Count++
		for _, change := range []struct {
			target *int64
			delta  int64
		}{
			{&stats.InputMinor, event.input}, {&stats.OutputMinor, event.output},
			{&stats.DiscountMinor, event.discount}, {&stats.AddonMinor, event.addon},
			{&aggregate.point.InputMinor, event.input}, {&aggregate.point.OutputMinor, event.output},
		} {
			if err := addStatisticAmount(change.target, change.delta); err != nil {
				return model.AdminStatistics{}, err
			}
		}
		aggregate.users[event.userID] = struct{}{}
	}
	stats.UniqueUsers = int64(len(users))
	var err error
	stats.NetMinor, err = statisticNetMinor(stats.InputMinor, stats.OutputMinor)
	if err != nil {
		return model.AdminStatistics{}, err
	}
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		point := buckets[key].point
		point.UniqueUsers = int64(len(buckets[key].users))
		point.NetMinor, err = statisticNetMinor(point.InputMinor, point.OutputMinor)
		if err != nil {
			return model.AdminStatistics{}, err
		}
		stats.Series = append(stats.Series, point)
	}
	return stats, nil
}

func statisticBucket(value time.Time, bucket string) string {
	if bucket != "weekly" {
		return value.Format(time.DateOnly)
	}
	weekday := (int(value.Weekday()) + 6) % 7
	return time.Date(value.Year(), value.Month(), value.Day()-weekday, 0, 0, 0, 0, value.Location()).Format(time.DateOnly)
}

func addStatisticAmount(total *int64, delta int64) error {
	if delta > 0 && *total > math.MaxInt64-delta || delta < 0 && *total < math.MinInt64-delta {
		return fmt.Errorf("statistic amount overflow: %w", ErrConflict)
	}
	*total += delta
	return nil
}

func statisticNetMinor(input, output int64) (int64, error) {
	if output > 0 && input < math.MinInt64+output || output < 0 && input > math.MaxInt64+output {
		return 0, fmt.Errorf("statistic net overflow: %w", ErrConflict)
	}
	return input - output, nil
}
