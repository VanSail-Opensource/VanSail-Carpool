package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/bits"
	"strconv"
	"time"
)

// TrafficUser is the narrow Proxy stream projection used by threshold scans.
type TrafficUser struct {
	ID                 int64
	UsedBytes          int64
	LimitBytes         int64
	ResetStrategy      string
	LastTrafficResetAt *time.Time
}

// ScanRepository persists reminder and traffic events transactionally.
type ScanRepository interface {
	EnqueueExpiryReminderNotifications(context.Context, time.Time) (int, error)
	EnqueueTrafficThresholdNotification(context.Context, string, int64, int64, string, *time.Time, time.Time) (bool, error)
	ProcessAutomaticTrafficResetObservation(context.Context, string, int64, int64, string, *time.Time, time.Time) (AutomaticResetResult, error)
}

// AutomaticResetResult distinguishes disabled accounts from handled reset periods.
type AutomaticResetResult struct {
	Handled      bool
	EventCreated bool
}

// TrafficRemote pages through the documented Proxy user stream.
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
	if s.logger != nil && (reminders > 0 || traffic > 0) {
		s.logger.Info("user notification scan queued events", "expiry_reminders", reminders, "traffic_thresholds", traffic)
	}
	return errors.Join(reminderErr, trafficErr)
}

func (s *Scanner) scanTraffic(ctx context.Context, now time.Time) (int, error) {
	const pageSize = 1000
	queued, cursor := 0, ""
	for {
		users, next, more, err := s.remote.ListNotificationUsers(ctx, cursor, pageSize)
		if err != nil {
			return queued, fmt.Errorf("list Proxy traffic users: %w", err)
		}
		for _, user := range users {
			if user.ID <= 0 || !aboveNinetyPercent(user.UsedBytes, user.LimitBytes) {
				continue
			}
			if aboveNinetyFivePercent(user.UsedBytes, user.LimitBytes) {
				result, resetErr := s.repository.ProcessAutomaticTrafficResetObservation(ctx, strconv.FormatInt(user.ID, 10),
					user.UsedBytes, user.LimitBytes, user.ResetStrategy, user.LastTrafficResetAt, now)
				if resetErr != nil {
					return queued, resetErr
				}
				if result.EventCreated {
					queued++
				}
				if result.Handled {
					continue
				}
			}
			inserted, err := s.repository.EnqueueTrafficThresholdNotification(ctx, strconv.FormatInt(user.ID, 10),
				user.UsedBytes, user.LimitBytes, user.ResetStrategy, user.LastTrafficResetAt, now)
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

func aboveNinetyFivePercent(used, limit int64) bool {
	if used < 0 || limit <= 0 {
		return false
	}
	usedHi, usedLo := bits.Mul64(uint64(used), 20)
	limitHi, limitLo := bits.Mul64(uint64(limit), 19)
	return usedHi > limitHi || usedHi == limitHi && usedLo > limitLo
}

func aboveNinetyPercent(used, limit int64) bool {
	if used < 0 || limit <= 0 {
		return false
	}
	usedHi, usedLo := bits.Mul64(uint64(used), 10)
	limitHi, limitLo := bits.Mul64(uint64(limit), 9)
	return usedHi > limitHi || usedHi == limitHi && usedLo > limitLo
}
