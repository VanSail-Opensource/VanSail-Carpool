package database

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type adminBulkQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// PreviewAdminBulkExtension evaluates active, inclusive-OR targets without mutation.
func (s *Store) PreviewAdminBulkExtension(ctx context.Context, filter AdminBulkExtensionFilter, now time.Time) (AdminBulkExtensionPreview, error) {
	_, preview, err := adminBulkTargets(ctx, s.db, filter, now)
	return preview, err
}

func adminBulkTargets(ctx context.Context, queryer adminBulkQueryer, filter AdminBulkExtensionFilter, now time.Time) ([]adminBulkTarget, AdminBulkExtensionPreview, error) {
	clause, args, err := adminBulkMatchClause(filter)
	if err != nil {
		return nil, AdminBulkExtensionPreview{}, err
	}
	queryArgs := []any{stamp(now), stamp(now)}
	queryArgs = append(queryArgs, args...)
	rows, err := queryer.QueryContext(ctx, `SELECT purchases.id,purchases.user_id FROM purchases
		WHERE purchases.status='active' AND purchases.valid_from<=? AND purchases.valid_until>? AND (`+clause+`)
		ORDER BY purchases.user_id,purchases.valid_until,purchases.id`, queryArgs...)
	if err != nil {
		return nil, AdminBulkExtensionPreview{}, err
	}
	targets := make([]adminBulkTarget, 0)
	seen := make(map[string]struct{})
	activeCount := 0
	for rows.Next() {
		var target adminBulkTarget
		if err := rows.Scan(&target.PurchaseID, &target.UserID); err != nil {
			_ = rows.Close()
			return nil, AdminBulkExtensionPreview{}, err
		}
		activeCount++
		if _, exists := seen[target.UserID]; !exists {
			seen[target.UserID] = struct{}{}
			targets = append(targets, target)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, AdminBulkExtensionPreview{}, err
	}
	_ = rows.Close()
	queued, err := countQueuedForTargets(ctx, queryer, targets)
	if err != nil {
		return nil, AdminBulkExtensionPreview{}, err
	}
	return targets, AdminBulkExtensionPreview{MatchedUsers: len(targets), ActivePurchases: activeCount, QueuedSuccessors: queued}, nil
}

func adminBulkMatchClause(filter AdminBulkExtensionFilter) (string, []any, error) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, len(filter.ComboIDs)+2*len(filter.AddonSquadUUIDs))
	if len(filter.ComboIDs) > 0 {
		conditions = append(conditions, `purchases.combo_id IN (`+placeholders(len(filter.ComboIDs))+`)`)
		for _, id := range filter.ComboIDs {
			args = append(args, id)
		}
	}
	if len(filter.AddonSquadUUIDs) > 0 {
		marks := placeholders(len(filter.AddonSquadUUIDs))
		conditions = append(conditions, `(purchases.entitlement_squad_uuids IS NULL AND purchases.entitlement_addon_squad_uuids IS NULL
			AND EXISTS (SELECT 1 FROM purchase_addons WHERE purchase_id=purchases.id AND group_id IN (`+marks+`)))`)
		for _, id := range filter.AddonSquadUUIDs {
			args = append(args, id)
		}
		conditions = append(conditions, `EXISTS (SELECT 1 FROM json_each(CASE
			WHEN purchases.entitlement_squad_uuids IS NOT NULL THEN purchases.entitlement_squad_uuids
			WHEN purchases.entitlement_addon_squad_uuids IS NOT NULL THEN purchases.entitlement_addon_squad_uuids
			ELSE '[]' END) WHERE value IN (`+marks+`))`)
		for _, id := range filter.AddonSquadUUIDs {
			args = append(args, id)
		}
	}
	if len(conditions) == 0 {
		return "", nil, ErrConflict
	}
	return strings.Join(conditions, " OR "), args, nil
}

func countQueuedForTargets(ctx context.Context, queryer adminBulkQueryer, targets []adminBulkTarget) (int, error) {
	if len(targets) == 0 {
		return 0, nil
	}
	args := make([]any, 0, len(targets))
	for _, target := range targets {
		args = append(args, target.UserID)
	}
	var count int
	if err := queryer.QueryRowContext(ctx, `SELECT COUNT(*) FROM purchases WHERE status='queued' AND user_id IN (`+
		placeholders(len(args))+`)`, args...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func placeholders(count int) string { return strings.TrimSuffix(strings.Repeat("?,", count), ",") }
