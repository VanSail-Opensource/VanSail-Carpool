package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var (
	ErrHourlySalesDisabled   = errors.New("hourly sales are disabled")
	ErrHourlyCooldown        = errors.New("hourly squad cooldown is active")
	ErrHourlyExtraDisabled   = errors.New("hourly extra traffic is not authorized")
	ErrHourlyNotExhausted    = errors.New("funded hourly traffic is not fully consumed")
	ErrHourlyDemandStale     = errors.New("hourly funding requires fresh pending payload")
	ErrHourlyDeliveryPending = errors.New("hourly native delivery proof is unavailable")
)

// HourlyActivationInput identifies one explicit activation and the reviewed
// terms. A changed offer/policy requires a new request, never a changed replay.
type HourlyActivationInput struct {
	OwnerID, GroupID, ActivationKey, ExpectedPolicyVersion string
	AccountGeneration, ExpectedOfferRevision               int64
	ActivationCode                                         string
}

// HourlyCommitInput binds the prepared native tuple to its original activation.
type HourlyCommitInput struct {
	HourlyActivationInput
	EnrollmentID, PeriodID  string
	ProxyUserID             int64
	ExpectedCompileRevision int64
	Delivery                HourlyDeliveryReady
}

// HourlyActivationResult returns an immutable receipt only after payment.
type HourlyActivationResult struct {
	Enrollment model.HourlyEnrollment
	Settlement *model.HourlySettlement
}

// HourlyOwnerSettlementResult retains deterministic charging order.
type HourlyOwnerSettlementResult struct {
	Enrollments          []model.HourlyEnrollment
	Settlements          []model.HourlySettlement
	SkippedEnrollmentIDs []string
}

// HourlyDeliveryReady is short-lived, server-only native readiness evidence.
// It is never persisted and may not be supplied by a member API request.
type HourlyDeliveryReady struct {
	EnrollmentID           string
	ObservedAt, ReadyUntil time.Time
	CompileRevision        int64
}

func hourlyActivationFingerprint(input HourlyActivationInput) (string, error) {
	if !validHourlyCreditName(input.OwnerID) || !validHourlyCreditName(input.GroupID) ||
		!validHourlyCreditName(input.ActivationKey) || input.AccountGeneration < 0 ||
		input.ExpectedOfferRevision <= 0 || !validHourlyCreditHex(input.ExpectedPolicyVersion, 64) {
		return "", ErrConflict
	}
	encoded, err := json.Marshal([]any{input.GroupID, input.ExpectedPolicyVersion, input.ExpectedOfferRevision})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func hourlyBillingPolicyTx(ctx context.Context, tx *sql.Tx, snapshot model.HourlyPolicySnapshot) error {
	if err := model.ValidateHourlyPolicy(snapshot.Policy); err != nil {
		return ErrConflict
	}
	version, err := lockHourlyPolicyVersionTx(ctx, tx)
	if err != nil {
		return err
	}
	if version == "" || version != snapshot.Version {
		return ErrConflict
	}
	return nil
}

func hourlyBillingClockTx(ctx context.Context, tx *sql.Tx) (time.Time, error) {
	var at time.Time
	err := tx.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&at)
	return at.UTC(), err
}

func enqueueHourlySyncTx(ctx context.Context, tx *sql.Tx, owner string, at time.Time) error {
	payload, err := json.Marshal(map[string]string{"userId": owner})
	if err != nil {
		return err
	}
	return insertOutboxTx(ctx, tx, "xnode_sync_user", string(payload), at, at)
}
