package database

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// BeginXNodePeriodClosure returns the exact durable generation it fenced. A
// stale caller cannot join a later closure, including one at the same timestamp.
func (s *Store) BeginXNodePeriodClosure(ctx context.Context, expected xnode.Period, at time.Time) (xnode.Period, error) {
	args := []any{expected.ID, expected.ProxyUserID, nullText(expected.PurchaseID), expected.ClosureGeneration}
	query := `SELECT ` + xnodePeriodColumns + ` FROM xnode_periods WHERE id=$1 AND proxy_user_id=$2 AND purchase_id IS NOT DISTINCT FROM $3 AND closure_generation=$4 AND status=$5`
	if expected.Status == "open" {
		if expected.ClosureGeneration == math.MaxInt64 {
			return xnode.Period{}, xnode.ErrConflict
		}
		query = `UPDATE xnode_periods SET status='closing',ended_at=$5,closure_generation=closure_generation+1 WHERE id=$1 AND proxy_user_id=$2 AND purchase_id IS NOT DISTINCT FROM $3 AND closure_generation=$4 AND status='open' RETURNING ` + xnodePeriodColumns
		args = append(args, stamp(at))
	} else if expected.Status == "closing" || expected.Status == "closed" {
		args = append(args, expected.Status)
	} else {
		return xnode.Period{}, xnode.ErrConflict
	}
	period, err := scanXNodePeriod(s.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, xnode.ErrNotFound) {
		err = xnode.ErrConflict
	}
	return period, err
}

// SealXNodeClosingPeriodNode serializes a committed participant barrier with
// reopen and rejects evidence captured for any other closure generation.
func (s *Store) SealXNodeClosingPeriodNode(ctx context.Context, expected xnode.Period, node string, barrier wire.TrafficBarrier) error {
	return s.sealPeriodNode(ctx, expected, node, barrier, closingPeriodSeal)
}
