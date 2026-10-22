package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

// AdminUserRecord is the list-safe user and aggregate balance projection.
type AdminUserRecord struct {
	User    model.User
	Balance model.Money
}

// AdminUserSearchFilter is the normalized, cursor-bound administrative search.
type AdminUserSearchFilter struct {
	Search     string
	State      string
	ComboIDs   []string
	SquadUUIDs []string
	Match      string
}

// ErrInvalidAdminUserSearch marks malformed admin-user search facets.
var ErrInvalidAdminUserSearch = errors.New("invalid admin user search")

// ListAdminUsersPage returns a stable cursor page with its balances in one query.
func (s *Store) ListAdminUsersPage(ctx context.Context, cursor string, filter AdminUserSearchFilter, limit int) ([]AdminUserRecord, *string, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	filter, err := normalizeAdminUserSearch(filter)
	if err != nil {
		return nil, nil, err
	}
	fingerprint, err := adminUserFilterFingerprint(filter)
	if err != nil {
		return nil, nil, err
	}
	query := `SELECT ` + userColumns + `,COALESCE(balances.txb_minor,0) FROM users
		LEFT JOIN balances ON balances.user_id=users.id WHERE 1=1`
	args := make([]any, 0, 20)
	if filter.Search != "" {
		pattern := "%" + escapeLike(filter.Search) + "%"
		query += ` AND (users.telegram_first_name LIKE ? ESCAPE '\' COLLATE NOCASE
			OR users.telegram_last_name LIKE ? ESCAPE '\' COLLATE NOCASE
			OR users.telegram_username LIKE ? ESCAPE '\' COLLATE NOCASE
			OR CAST(users.telegram_id AS TEXT) LIKE ? ESCAPE '\' COLLATE NOCASE
			OR CAST(users.telegram_id AS TEXT) LIKE ? ESCAPE '\'
			OR users.id LIKE ? ESCAPE '\' COLLATE NOCASE
			OR COALESCE(users.remna_user_id,'') LIKE ? ESCAPE '\' COLLATE NOCASE)`
		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if clause, clauseArgs := adminUserFacetClause(filter, time.Now().UTC()); clause != "" {
		query += " AND (" + clause + ")"
		args = append(args, clauseArgs...)
	}
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, fingerprint)
		if err != nil {
			return nil, nil, err
		}
		query += ` AND (users.created_at<? OR (users.created_at=? AND users.id<?))`
		args = append(args, decoded.Timestamp, decoded.Timestamp, decoded.ID)
	}
	query += ` ORDER BY users.created_at DESC,users.id DESC LIMIT ?`
	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list admin user page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminUserRecord, 0, limit+1)
	for rows.Next() {
		var balanceMinor int64
		user, scanErr := scanUserWith(rows, &balanceMinor)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, AdminUserRecord{User: user, Balance: model.TXBMoney(balanceMinor)})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(items) <= limit {
		return items, nil, nil
	}
	items = items[:limit]
	last := items[len(items)-1].User
	next, err := encodeTimestampCursor(last.CreatedAt, last.ID, fingerprint)
	return items, &next, err
}

func normalizeAdminUserSearch(filter AdminUserSearchFilter) (AdminUserSearchFilter, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	filter.State = strings.ToLower(strings.TrimSpace(filter.State))
	filter.Match = strings.ToLower(strings.TrimSpace(filter.Match))
	if filter.Match == "" {
		filter.Match = "and"
	}
	if len(filter.Search) > 100 || (filter.State != "" && filter.State != "active" && filter.State != "non_active") ||
		(filter.Match != "and" && filter.Match != "or") {
		return AdminUserSearchFilter{}, ErrInvalidAdminUserSearch
	}
	var err error
	if filter.ComboIDs, err = normalizeAdminSearchIDs(filter.ComboIDs, false); err != nil {
		return AdminUserSearchFilter{}, err
	}
	if filter.SquadUUIDs, err = normalizeAdminSearchIDs(filter.SquadUUIDs, true); err != nil {
		return AdminUserSearchFilter{}, err
	}
	return filter, nil
}

func normalizeAdminSearchIDs(values []string, lower bool) ([]string, error) {
	if len(values) > 50 {
		return nil, ErrInvalidAdminUserSearch
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" || len(value) > 160 {
			return nil, ErrInvalidAdminUserSearch
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result, nil
}

func adminUserFilterFingerprint(filter AdminUserSearchFilter) (string, error) {
	payload, err := json.Marshal(filter)
	if err != nil {
		return "", fmt.Errorf("encode admin user filter: %w", err)
	}
	return pageFilterFingerprint(string(payload)), nil
}

func adminUserFacetClause(filter AdminUserSearchFilter, now time.Time) (string, []any) {
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 8+len(filter.ComboIDs)+2*len(filter.SquadUUIDs))
	active := `purchases.status IN ('active','activating') AND purchases.valid_from<=? AND purchases.valid_until>?`
	if filter.State == "active" {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM purchases WHERE purchases.user_id=users.id AND `+active+`)`)
		args = append(args, stamp(now), stamp(now))
	} else if filter.State == "non_active" {
		conditions = append(conditions, `NOT EXISTS (SELECT 1 FROM purchases WHERE purchases.user_id=users.id AND `+active+`)`)
		args = append(args, stamp(now), stamp(now))
	}
	if len(filter.ComboIDs) > 0 {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM purchases WHERE purchases.user_id=users.id AND purchases.combo_id IN (`+placeholders(len(filter.ComboIDs))+`))`)
		for _, id := range filter.ComboIDs {
			args = append(args, id)
		}
	}
	if len(filter.SquadUUIDs) > 0 {
		marks := placeholders(len(filter.SquadUUIDs))
		conditions = append(conditions, `EXISTS (SELECT 1 FROM purchases WHERE purchases.user_id=users.id AND (
			(purchases.entitlement_squad_uuids IS NULL AND purchases.entitlement_addon_squad_uuids IS NULL AND EXISTS (
				SELECT 1 FROM purchase_addons WHERE purchase_id=purchases.id AND lower(remna_squad_uuid) IN (`+marks+`))) OR
			EXISTS (SELECT 1 FROM json_each(CASE WHEN purchases.entitlement_squad_uuids IS NOT NULL THEN purchases.entitlement_squad_uuids
				WHEN purchases.entitlement_addon_squad_uuids IS NOT NULL THEN purchases.entitlement_addon_squad_uuids ELSE '[]' END)
				WHERE lower(value) IN (`+marks+`))))`)
		for _, id := range filter.SquadUUIDs {
			args = append(args, id)
		}
		for _, id := range filter.SquadUUIDs {
			args = append(args, id)
		}
	}
	return strings.Join(conditions, " "+strings.ToUpper(filter.Match)+" "), args
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
