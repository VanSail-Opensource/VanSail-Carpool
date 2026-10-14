package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"sort"
	"strings"
)

// ComboByID returns one live combo with UUID-only included-squad placeholders.
func (s *Store) ComboByID(ctx context.Context, id string, activeOnly bool) (model.Combo, error) {
	query := comboSelect + ` WHERE id=?`
	if activeOnly {
		query += ` AND active=1`
	}
	return scanCombo(s.db.QueryRowContext(ctx, query, id))
}

// ListCombos returns the catalog in stable display order.
func (s *Store) ListCombos(ctx context.Context, activeOnly bool) ([]model.Combo, error) {
	query := comboSelect
	if activeOnly {
		query += ` WHERE active=1`
	}
	query += ` ORDER BY price_txb_minor,name,id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list combos: %w", err)
	}
	defer func() { _ = rows.Close() }()
	combos := make([]model.Combo, 0)
	for rows.Next() {
		combo, scanErr := scanCombo(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		combos = append(combos, combo)
	}
	return combos, rows.Err()
}

const comboSelect = `SELECT id,name,description,price_txb_minor,validity_days,traffic_limit_bytes,reset_strategy,active,rollover_min_remaining_bps,included_squad_uuids,created_at,updated_at,ip_lookup_quota FROM combos`

func scanCombo(row rowScanner) (model.Combo, error) {
	var combo model.Combo
	var active int
	var quota sql.NullInt64
	var encodedSquads, created, updated string
	if err := row.Scan(&combo.ID, &combo.Name, &combo.Description, &combo.PriceTXBMinor, &combo.ValidityDays,
		&combo.TrafficLimitBytes, &combo.ResetStrategy, &active, &combo.RolloverMinRemainingBPS,
		&encodedSquads, &created, &updated, &quota); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Combo{}, ErrNotFound
		}
		return model.Combo{}, fmt.Errorf("scan combo: %w", err)
	}
	var squadUUIDs []string
	if err := json.Unmarshal([]byte(encodedSquads), &squadUUIDs); err != nil {
		return model.Combo{}, fmt.Errorf("decode combo squad UUIDs: %w", err)
	}
	combo.IncludedSquads = make([]model.SquadProduct, 0, len(squadUUIDs))
	for _, uuid := range uniqueSorted(squadUUIDs) {
		combo.IncludedSquads = append(combo.IncludedSquads, model.SquadProduct{ID: uuid, RemnaSquadUUID: uuid, UpstreamPresent: true,
			AccessibleNodes: []model.CatalogNode{}})
	}
	combo.Active = active == 1
	combo.IPLookupQuota = intPointer(quota)
	combo.Price = model.TXBMoney(combo.PriceTXBMinor)
	combo.TrafficLimit = fmt.Sprintf("%d", combo.TrafficLimitBytes)
	var err error
	combo.CreatedAt, err = parseStamp(created)
	if err != nil {
		return model.Combo{}, err
	}
	combo.UpdatedAt, err = parseStamp(updated)
	return combo, err
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if _, ok := seen[value]; ok || value == "" {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func intPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}
