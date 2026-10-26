package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

func pricingPublicationFrozenTx(ctx context.Context, tx *sql.Tx, p pricing.Policy, now time.Time) (bool, error) {
	var count int
	err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_admin_overrides WHERE status='ACTIVE' AND starts_at<=? AND expires_at>? AND command IN ('FREEZE','MANUAL_RATE') AND (scope_type='global' OR scope_type='product' AND scope_id=?)`, pricingStamp(now), pricingStamp(now), p.ProductType+":"+p.ProductID).Scan(&count)
	if err != nil || count > 0 {
		return count > 0, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_admin_overrides o JOIN pricing_model_versions m ON m.id=o.scope_id WHERE o.scope_type='model' AND o.command='FREEZE' AND o.status='ACTIVE' AND o.starts_at<=? AND o.expires_at>? AND (m.kind='traffic' OR m.kind=? OR m.kind=?)`, pricingStamp(now), pricingStamp(now), "response:"+p.ProductType+":"+p.ProductID, "renewal:"+p.ProductID).Scan(&count); err != nil || count > 0 {
		return count > 0, err
	}
	nodes, err := pricingNodesTx(ctx, tx, p.ProductType, p.ProductID)
	if err != nil {
		return false, err
	}
	for _, node := range nodes {
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_admin_overrides WHERE scope_type IN ('node','machine') AND scope_id=? AND command='FREEZE' AND status='ACTIVE' AND starts_at<=? AND expires_at>?`, node, pricingStamp(now), pricingStamp(now)).Scan(&count); err != nil || count > 0 {
			return count > 0, err
		}
	}
	return false, nil
}
