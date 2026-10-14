package database

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/google/uuid"
)

const connectivityRetention = 24 * time.Hour
const connectivityTimestampLayout = "2006-01-02T15:04:05.000000000Z"

var connectivityHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
var connectivityCodePattern = regexp.MustCompile(`^CONNECTIVITY_[A-Z0-9_]{1,80}$`)
var connectivityHostPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var _ connectivity.Repository = (*Store)(nil)

// BeginConnectivityAttempt durably records a running attempt before executing a probe.
func (s *Store) BeginConnectivityAttempt(ctx context.Context, attempt connectivity.Attempt) error {
	if !cursorIDPattern.MatchString(attempt.ID) || !cursorIDPattern.MatchString(attempt.RunID) ||
		(attempt.HostUUID != "" && (!connectivityHostPattern.MatchString(attempt.HostUUID) || attempt.HostUUID == uuid.Nil.String())) ||
		!connectivityHashPattern.MatchString(attempt.ConfigHash) || attempt.RemnawaveUserID <= 0 ||
		(attempt.Trigger != "manual" && attempt.Trigger != "scheduled") || attempt.StartedAt.IsZero() {
		return ErrConflict
	}
	var hostUUID any
	if attempt.HostUUID != "" {
		hostUUID = attempt.HostUUID
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	result, err := s.db.ExecContext(ctx, `INSERT INTO host_connectivity_attempts
		(id,run_id,host_uuid,config_hash,remnawave_user_id,trigger,started_at,status)
		VALUES(?,?,?,?,?,?,?,'running') ON CONFLICT(id) DO NOTHING`, attempt.ID, attempt.RunID,
		hostUUID, attempt.ConfigHash, attempt.RemnawaveUserID, attempt.Trigger, connectivityStamp(attempt.StartedAt))
	if err != nil {
		return fmt.Errorf("begin connectivity attempt: %w", err)
	}
	return connectivityChanged(result.RowsAffected())
}

// FinishConnectivityAttempt seals a running attempt once, without replacing terminal evidence.
func (s *Store) FinishConnectivityAttempt(ctx context.Context, id string, outcome connectivity.Outcome, now time.Time) error {
	if !cursorIDPattern.MatchString(id) || now.IsZero() || !validConnectivityOutcome(outcome) {
		return ErrConflict
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	at := connectivityStamp(now)
	result, err := s.db.ExecContext(ctx, `UPDATE host_connectivity_attempts SET finished_at=?,status=?,
		latency_ms=?,http_status=?,error_code=? WHERE id=? AND status='running' AND started_at<=?`,
		at, outcome.Status, outcome.LatencyMS, outcome.HTTPStatus, outcome.ErrorCode, id, at)
	if err != nil {
		return fmt.Errorf("finish connectivity attempt: %w", err)
	}
	return connectivityChanged(result.RowsAffected())
}

func validConnectivityOutcome(outcome connectivity.Outcome) bool {
	switch outcome.Status {
	case "connected", "failed", "unsupported", "error", "interrupted":
	default:
		return false
	}
	if outcome.ErrorCode != "" && !connectivityCodePattern.MatchString(outcome.ErrorCode) {
		return false
	}
	if outcome.LatencyMS != nil && (math.IsNaN(*outcome.LatencyMS) || math.IsInf(*outcome.LatencyMS, 0) || *outcome.LatencyMS < 0) {
		return false
	}
	return outcome.HTTPStatus == nil || (*outcome.HTTPStatus >= 100 && *outcome.HTTPStatus <= 599)
}

func connectivityChanged(count int64, err error) error {
	if err != nil {
		return fmt.Errorf("count connectivity transition: %w", err)
	}
	if count != 1 {
		return ErrConflict
	}
	return nil
}

// Fixed-width UTC nanoseconds allow indexed, exact chronological comparisons.
func connectivityStamp(at time.Time) string {
	return at.UTC().Format(connectivityTimestampLayout)
}
