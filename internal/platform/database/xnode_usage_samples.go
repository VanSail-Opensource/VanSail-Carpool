package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func rawUsageValue(from, through time.Time, generation int64, up, down uint64) *usageMinuteValue {
	value := &usageMinuteValue{generation: generation, from: from.UTC(), through: through.UTC()}
	value.upload.SetUint64(up)
	value.download.SetUint64(down)
	return value
}

// Combo attribution follows the original period/purchase, never the currently
// selected purchase or current account generation. Service/PAYG/hourly periods
// have no purchase and remain on their independent paths.
func recordComboUsageTx(ctx context.Context, tx *sql.Tx, node string, sample wire.TrafficSample, period nativePeriodValue, buffer *nativeReportBuffer) error {
	if period.historyOwner == "" || period.purchase == "" {
		return nil
	}
	if sample.Through.After(time.Now().Add(5*time.Minute)) || sample.From.After(sample.Through) {
		return xnode.ErrInvalid
	}
	if sample.Through.Before(time.Now().UTC().Add(-xnode.PAYGWindow - time.Minute)) {
		return nil
	}
	key := usageMinuteKey{owner: period.historyOwner, mode: "combo", reference: period.purchase, principal: int64(sample.UserID), period: sample.AccountingPeriod, node: node, minute: stamp(sample.Through.UTC().Truncate(time.Minute))}
	return stageUsageMinute(ctx, tx, buffer, key, rawUsageValue(sample.From, sample.Through, -1, sample.UploadBytes, sample.DownloadBytes))
}

func recordHourlyUsageTx(ctx context.Context, tx *sql.Tx, node string, sample wire.HourlySample, buffer *nativeReportBuffer) error {
	if wire.ValidateHourlySample(sample) != nil || sample.From.UTC().Format(time.DateOnly) != sample.DayUTC || sample.Through.UTC().Format(time.DateOnly) != sample.DayUTC || sample.Through.After(time.Now().Add(5*time.Minute)) {
		return xnode.ErrInvalid
	}
	scope, err := hourlyUsageScopeTx(ctx, tx, buffer, hourlyUsageKey{sample.EnrollmentID, node})
	if err != nil {
		return err
	}
	if scope.owner != sample.Owner.ID || scope.generation < 0 || uint64(scope.generation) != sample.Owner.Generation || scope.principal != int64(sample.UserID) || scope.period != sample.AccountingPeriod || !scope.paid {
		return xnode.ErrInvalid
	}
	if sample.UploadBytes == 0 && sample.DownloadBytes == 0 || sample.Through.Before(time.Now().UTC().Add(-xnode.PAYGWindow-time.Minute)) {
		return nil
	}
	key := usageMinuteKey{owner: scope.owner, mode: "hourly", reference: sample.EnrollmentID, principal: scope.principal, period: scope.period, node: node, minute: stamp(sample.Through.UTC().Truncate(time.Minute))}
	return stageUsageMinute(ctx, tx, buffer, key, rawUsageValue(sample.From, sample.Through, scope.generation, sample.UploadBytes, sample.DownloadBytes))
}
