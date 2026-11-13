package database

import (
	"context"
	"database/sql"
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
	User            model.User
	Balance         model.Money
	Synchronization AdminUserSynchronization
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
	query := `SELECT ` + userColumns + `,COALESCE(balances.txb_minor,0),` + adminUserSynchronizationColumns + ` FROM users
		LEFT JOIN balances ON balances.user_id=users.id
		LEFT JOIN xnode_identities native ON native.user_id=users.id WHERE users.account_status='active'`
	args := make([]any, 0, 20)
	if filter.Search != "" {
		pattern := "%" + escapeLike(filter.Search) + "%"
		query += fmt.Sprintf(" AND (users.telegram_first_name ILIKE $%d ESCAPE '\\' \n\t\t\tOR users.telegram_last_name ILIKE $%d ESCAPE '\\' \n\t\t\tOR users.telegram_username ILIKE $%d ESCAPE '\\' \n\t\t\tOR CAST(users.telegram_id AS TEXT) ILIKE $%d ESCAPE '\\'\n\t\t\tOR users.id ILIKE $%d ESCAPE '\\' \n\t\t\tOR COALESCE(users.proxy_user_id,'') ILIKE $%d ESCAPE '\\' )", len(args)+1, len(args)+2, len(args)+3, len(args)+4, len(args)+5, len(args)+6)

		args = append(args, pattern, pattern, pattern, pattern, pattern, pattern)
	}
	if clause, clauseArgs := adminUserFacetClause(filter, time.Now().UTC(), len(args)+1); clause != "" {
		query += " AND (" + clause + ")"
		args = append(args, clauseArgs...)
	}
	orderedCreated := purchaseTimestampOrderSQL("users.created_at")
	if cursor != "" {
		decoded, err := decodeTimestampCursor(cursor, fingerprint)
		if err != nil {
			return nil, nil, err
		}
		orderedCursor := purchaseTimestampOrderSQL(fmt.Sprintf("$%d", len(args)+1))
		query += fmt.Sprintf(" AND (%s,users.id)<(%s,$%d)", orderedCreated, orderedCursor, len(args)+2)
		args = append(args, decoded.Timestamp, decoded.ID)
	}
	query += fmt.Sprintf(" ORDER BY %s DESC,users.id DESC LIMIT $%d", orderedCreated, len(args)+1)

	args = append(args, limit+1)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list admin user page: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]AdminUserRecord, 0, limit+1)
	for rows.Next() {
		var balanceMinor int64
		var nativeID sql.NullInt64
		var credentialsReady, applied bool
		user, scanErr := scanUserWith(rows, &balanceMinor, &nativeID, &credentialsReady, &applied)
		if scanErr != nil {
			return nil, nil, scanErr
		}
		items = append(items, AdminUserRecord{User: user, Balance: model.TXBMoney(balanceMinor),
			Synchronization: adminUserSynchronizationFor(user, nativeID, credentialsReady, applied)})
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

func adminUserFacetClause(filter AdminUserSearchFilter, now time.Time, first ...int) (string, []any) {
	start := 1
	if len(first) > 0 {
		start = first[0]
	}
	conditions := make([]string, 0, 3)
	args := make([]any, 0, 8+len(filter.ComboIDs)+2*len(filter.SquadUUIDs))
	active := fmt.Sprintf(`purchases.status IN ('active','activating') AND `+purchaseTimestampOrderSQL("purchases.valid_from")+`<=$%d AND `+purchaseTimestampOrderSQL("purchases.valid_until")+`>$%d`, start, start+1)
	if filter.State == "active" {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM purchases WHERE purchases.user_id=users.id AND `+active+`)`)
		args = append(args, purchaseTimestampOrder(now), purchaseTimestampOrder(now))
	} else if filter.State == "non_active" {
		conditions = append(conditions, `NOT EXISTS (SELECT 1 FROM purchases WHERE purchases.user_id=users.id AND `+active+`)`)
		args = append(args, purchaseTimestampOrder(now), purchaseTimestampOrder(now))
	}
	if len(filter.ComboIDs) > 0 {
		conditions = append(conditions, `EXISTS (SELECT 1 FROM purchases WHERE purchases.user_id=users.id AND purchases.combo_id IN (`+placeholders(len(filter.ComboIDs), start+len(args))+`))`)
		for _, id := range filter.ComboIDs {
			args = append(args, id)
		}
	}
	if len(filter.SquadUUIDs) > 0 {
		marks := placeholders(len(filter.SquadUUIDs), start+len(args))
		secondMarks := placeholders(len(filter.SquadUUIDs), start+len(args)+len(filter.SquadUUIDs))
		conditions = append(conditions, `EXISTS (SELECT 1 FROM purchases WHERE purchases.user_id=users.id AND (
			(purchases.entitlement_squad_uuids IS NULL AND purchases.entitlement_addon_squad_uuids IS NULL AND EXISTS (
				SELECT 1 FROM purchase_addons WHERE purchase_id=purchases.id AND lower(group_id) IN (`+marks+`))) OR
			EXISTS (SELECT 1 FROM jsonb_array_elements_text((CASE WHEN purchases.entitlement_squad_uuids IS NOT NULL THEN purchases.entitlement_squad_uuids
				WHEN purchases.entitlement_addon_squad_uuids IS NOT NULL THEN purchases.entitlement_addon_squad_uuids ELSE '[]' END)::jsonb)
				WHERE lower(value) IN (`+secondMarks+`))))`)
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
