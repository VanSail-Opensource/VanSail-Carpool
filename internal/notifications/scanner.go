package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/bits"
	"time"
)

// TrafficUser captures one native owner's current billing period for threshold scans.
type TrafficUser struct {
	ID                 int64
	OwnerID            string
	AccountGeneration  int64
	PeriodID           string
	UsedBytes          int64
	LimitBytes         int64
	ResetStrategy      string
	LastTrafficResetAt *time.Time
}

// ScanRepository persists reminder and traffic events transactionally.
type ScanRepository interface {
	EnqueueExpiryReminderNotifications(context.Context, time.Time) (int, error)
	EnqueueTrafficThresholdNotification(context.Context, TrafficUser, time.Time) (bool, error)
}

// HourlyFundingScanner is optional so existing ScanRepository implementations
// keep compiling without hourly funding support.
type HourlyFundingScanner interface {
	EnqueueHourlyFundingWarnings(context.Context, time.Time) (int, error)
}

// TrafficRemote pages through current native owners and their paid periods.
type TrafficRemote interface {
	ListNotificationUsers(context.Context, string, int) ([]TrafficUser, *string, bool, error)
}

// Scanner evaluates time and traffic thresholds without sending directly.
type Scanner struct {
	repository ScanRepository
	remote     TrafficRemote
	logger     *slog.Logger
}

// NewScanner creates the five-minute notification scanner.
func NewScanner(repository ScanRepository, remote TrafficRemote, logger *slog.Logger) *Scanner {
	return &Scanner{repository: repository, remote: remote, logger: logger}
}

// Scan enqueues every newly eligible event once.
func (s *Scanner) Scan(ctx context.Context, now time.Time) error {
	reminders, reminderErr := s.repository.EnqueueExpiryReminderNotifications(ctx, now.UTC())
	traffic, trafficErr := s.scanTraffic(ctx, now.UTC())
	warnings, warningErr := s.scanHourlyFunding(ctx, now.UTC())
	if s.logger != nil && (reminders > 0 || traffic > 0 || warnings > 0) {
		s.logger.Info("user notification scan queued events", "expiry_reminders", reminders,
			"traffic_thresholds", traffic, "hourly_funding_warnings", warnings)
	}
	return errors.Join(reminderErr, trafficErr, warningErr)
}

func (s *Scanner) scanHourlyFunding(ctx context.Context, now time.Time) (int, error) {
	scanner, ok := s.repository.(HourlyFundingScanner)
	if !ok {
		return 0, nil
	}
	return scanner.EnqueueHourlyFundingWarnings(ctx, now)
}

func (s *Scanner) scanTraffic(ctx context.Context, now time.Time) (int, error) {
	const pageSize = 1000
	queued, cursor := 0, ""
	for {
		users, next, more, err := s.remote.ListNotificationUsers(ctx, cursor, pageSize)
		if err != nil {
			return queued, fmt.Errorf("list native traffic users: %w", err)
		}
		for _, user := range users {
			if user.ID <= 0 || !aboveNinetyPercent(user.UsedBytes, user.LimitBytes) {
				continue
			}
			inserted, err := s.repository.EnqueueTrafficThresholdNotification(ctx, user, now)
			if err != nil {
				return queued, err
			}
			if inserted {
				queued++
			}
		}
		if !more || next == nil || *next == "" || *next == cursor {
			return queued, nil
		}
		cursor = *next
	}
}

func aboveNinetyPercent(used, limit int64) bool {
	if used < 0 || limit <= 0 {
		return false
	}
	usedHi, usedLo := bits.Mul64(uint64(used), 10)
	limitHi, limitLo := bits.Mul64(uint64(limit), 9)
	return usedHi > limitHi || usedHi == limitHi && usedLo > limitLo
}
