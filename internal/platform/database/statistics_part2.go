package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"sort"
	"time"
)

func (s *Store) LuckyDrawStatistics(ctx context.Context, drawID string, from, to time.Time, bucket string, location *time.Location) (model.AdminStatistics, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id,fee_minor,reward_kind,reward_payload,created_at FROM activity_draw_results WHERE draw_id=? AND created_at>=? AND created_at<? ORDER BY created_at`, drawID, stamp(from), stamp(to))
	if err != nil {
		return model.AdminStatistics{}, err
	}
	events, err := scanStatisticEvents(rows, func(rows *sql.Rows) (statisticEvent, error) {
		var event statisticEvent
		var kind activity.RewardKind
		var payload, created string
		err := rows.Scan(&event.userID, &event.input, &kind, &payload, &created)
		if err == nil && kind == activity.RewardTXBDelta {
			var reward activity.Reward
			err = json.Unmarshal([]byte(payload), &reward)
			event.output = reward.TXBDeltaMinor
		}
		if err == nil {
			event.created, err = parseStamp(created)
		}
		return event, err
	})
	if err != nil {
		return model.AdminStatistics{}, err
	}
	distributionRows, err := s.db.QueryContext(ctx, `SELECT prize_id,prize_name,COUNT(*) FROM activity_draw_results WHERE draw_id=? AND created_at>=? AND created_at<? GROUP BY prize_id,prize_name ORDER BY COUNT(*) DESC,prize_name`, drawID, stamp(from), stamp(to))
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
	return aggregateStatistics(drawID, from, to, bucket, location, events, distribution), nil
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

func aggregateStatistics(resourceID string, from, to time.Time, bucket string, location *time.Location, events []statisticEvent, distribution []model.StatisticSlice) model.AdminStatistics {
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
		stats.InputMinor += event.input
		stats.OutputMinor += event.output
		stats.DiscountMinor += event.discount
		stats.AddonMinor += event.addon
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
		aggregate.point.InputMinor += event.input
		aggregate.point.OutputMinor += event.output
		aggregate.users[event.userID] = struct{}{}
	}
	stats.UniqueUsers = int64(len(users))
	stats.NetMinor = stats.InputMinor - stats.OutputMinor
	keys := make([]string, 0, len(buckets))
	for key := range buckets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		point := buckets[key].point
		point.UniqueUsers = int64(len(buckets[key].users))
		point.NetMinor = point.InputMinor - point.OutputMinor
		stats.Series = append(stats.Series, point)
	}
	return stats
}

func statisticBucket(value time.Time, bucket string) string {
	if bucket != "weekly" {
		return value.Format(time.DateOnly)
	}
	weekday := (int(value.Weekday()) + 6) % 7
	return time.Date(value.Year(), value.Month(), value.Day()-weekday, 0, 0, 0, 0, value.Location()).Format(time.DateOnly)
}
