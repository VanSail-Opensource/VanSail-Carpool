package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"sort"
)

func applyXNodeTrafficTx(ctx context.Context, tx *sql.Tx, nodeID string, report wire.TrafficReport, batch *trafficBatchState, native *nativeReportBuffer) error {
	samples := report.Samples
	var err error
	sampled := map[int]bool{}
	if batch == nil {
		paygIDs := map[int64]bool{}
		for _, sample := range samples {
			paygIDs[int64(sample.UserID)] = true
		}
		if err = lockPAYGOwnersTx(ctx, tx, nodeID, paygIDs); err != nil {
			return err
		}
		periods := map[string]bool{}
		for _, sample := range samples {
			periods[sample.AccountingPeriod] = true
		}
		ordered := make([]string, 0, len(periods))
		for period := range periods {
			ordered = append(ordered, period)
		}
		sort.Strings(ordered)
		for _, period := range ordered {
			var locked string
			if err = tx.QueryRowContext(ctx, `SELECT id FROM xnode_periods WHERE id=$1 FOR UPDATE`, period).Scan(&locked); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return xnode.NewTrafficPeriodMissing(period)
				}
				return err
			}
		}
	}
	for _, sample := range samples {
		sampled[sample.UserID] = true
		if batch == nil || batch.payg[int64(sample.UserID)] {
			if handled, e := recordPAYGSampleTx(ctx, tx, nodeID, sample, native); e != nil {
				return e
			} else if handled {
				continue
			}
		}
		// Acceptance time determines retirement: even offline backlog adds raw
		// bytes at 1x, retaining the existing historical fractional remainder.
		if err = recordXNodeSample(ctx, tx, nodeID, xnode.MultiplierScale, sample, native); err != nil {
			return err
		}
	}
	for user, bytes := range report.Traffic {
		if sampled[user] || bytes == [2]uint64{} {
			continue
		}
		var service sql.NullString
		var metered, migrated bool
		if err = tx.QueryRowContext(ctx, `SELECT service_name,
			EXISTS(SELECT 1 FROM xnode_metered_probes p WHERE p.proxy_user_id=i.proxy_user_id),
			EXISTS(SELECT 1 FROM member_identity_aliases a WHERE (a.old_proxy_id=$1 OR a.proxy_id=$2) AND a.proxy_id=i.proxy_user_id AND a.user_id=i.user_id)
			FROM xnode_identities i WHERE proxy_user_id=COALESCE((SELECT proxy_id FROM member_identity_aliases WHERE old_proxy_id=$3),$4)`, user, user, user, user).Scan(&service, &metered, &migrated); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return xnode.ErrTrafficIdentityMissing
			}
			return err
		}
		if (!service.Valid && !migrated) || metered {
			return xnode.ErrTrafficSampleMissing
		}
	}
	for _, sample := range report.HourlySamples {
		if err = recordHourlyUsageTx(ctx, tx, nodeID, sample, native); err != nil {
			return err
		}
	}
	return nil
}
