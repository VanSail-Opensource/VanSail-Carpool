package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func accountPurgeActivityScopesTx(ctx context.Context, tx *sql.Tx, target model.AccountPurge) ([]xnode.PurgeActivityScope, error) {
	rows, err := tx.QueryContext(ctx, `SELECT p.proxy_user_id,p.id FROM xnode_periods p WHERE `+accountNativePeriodScope+`
 ORDER BY p.proxy_user_id,p.id LIMIT 100001`, target.UserID, target.ThroughGeneration)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scopes := []xnode.PurgeActivityScope{}
	for rows.Next() {
		var scope xnode.PurgeActivityScope
		if err = rows.Scan(&scope.PrincipalID, &scope.PeriodID); err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	if len(scopes) > 100000 {
		return nil, fmt.Errorf("ACCOUNT_PURGE_ACTIVITY_CAPACITY: %w", xnode.ErrAccessAccountingPending)
	}
	return scopes, rows.Err()
}
