package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
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

const comboSelect = `SELECT id,name,description,price_txb_minor,validity_days,traffic_limit_bytes,reset_strategy,active,included_squad_uuids,created_at,updated_at,ip_lookup_quota,whitelist_ip_quota FROM combos`

func scanCombo(row rowScanner) (model.Combo, error) {
	var combo model.Combo
	var active int
	var quota sql.NullInt64
	var encodedSquads, created, updated string
	if err := row.Scan(&combo.ID, &combo.Name, &combo.Description, &combo.PriceTXBMinor, &combo.ValidityDays,
		&combo.TrafficLimitBytes, &combo.ResetStrategy, &active,
		&encodedSquads, &created, &updated, &quota, &combo.WhitelistIPQuota); err != nil {
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
		combo.IncludedSquads = append(combo.IncludedSquads, model.SquadProduct{ID: uuid, GroupID: uuid, UpstreamPresent: true,
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

const squadSelect = `SELECT group_id,description,profile_json,price_txb_minor,visible,stock_limit,activation_required,created_at,updated_at,billing_mode,payg_price_minor,payg_state FROM squad_product_overrides`

func scanSquad(row rowScanner) (model.SquadProduct, error) {
	var product model.SquadProduct
	var visible, activationRequired int
	var created, updated string
	var stockLimit sql.NullInt64
	var profileJSON sql.NullString
	if err := row.Scan(&product.GroupID, &product.Description, &profileJSON, &product.PriceTXBMinor, &visible, &stockLimit, &activationRequired, &created, &updated, &product.BillingMode, &product.PAYGPriceTXBMinor, &product.PAYGState); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.SquadProduct{}, ErrNotFound
		}
		return model.SquadProduct{}, fmt.Errorf("scan squad override: %w", err)
	}
	product.ID = product.GroupID
	product.Visible = visible == 1
	product.ActivationRequired = activationRequired == 1
	product.UpstreamPresent = true
	product.StockLimit = intPointer(stockLimit)
	product.Price = model.TXBMoney(product.PriceTXBMinor)
	paygPrice := model.TXBMoney(product.PAYGPriceTXBMinor)
	product.PAYGPrice = &paygPrice
	product.AccessibleNodes = []model.CatalogNode{}
	var err error
	product.Profile, err = squadprofile.ParseJSON(profileJSON.String)
	if err != nil {
		return model.SquadProduct{}, err
	}
	product.CreatedAt, err = parseStamp(created)
	if err != nil {
		return model.SquadProduct{}, err
	}
	product.UpdatedAt, err = parseStamp(updated)
	return product, err
}
