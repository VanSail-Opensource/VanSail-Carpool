package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"time"
)

// SaveSquadProduct upserts only non-default local merchandising. Restoring all
// defaults removes the row so unedited upstream squads consume no local space.
func (s *Store) SaveSquadProduct(ctx context.Context, input SquadProductInput) (model.SquadProduct, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	uuid := strings.TrimSpace(input.GroupID)
	if input.PAYGState == "" {
		input.PAYGState = "normal"
		if existing, err := s.SquadProductByGroupID(ctx, uuid); err == nil {
			input.PAYGState = existing.PAYGState
		} else if !errors.Is(err, ErrNotFound) {
			return model.SquadProduct{}, err
		}
	}
	if input.PAYGState != "normal" && input.PAYGState != "hidden" && input.PAYGState != "unavailable" {
		return model.SquadProduct{}, ErrConflict
	}
	if input.BillingMode == "" {
		input.BillingMode = "combo"
	}
	if (input.BillingMode != "combo" && input.BillingMode != "payg") || input.PAYGPriceTXBMinor < 0 || (input.BillingMode == "payg" && input.PAYGPriceTXBMinor == 0) {
		return model.SquadProduct{}, ErrConflict
	}
	if uuid == "" || !input.UpstreamPresent {
		return model.SquadProduct{}, ErrNotFound
	}
	now := time.Now().UTC()
	if input.BillingMode == "combo" && input.PAYGPriceTXBMinor == 0 && strings.TrimSpace(input.Description) == "" && input.PriceTXBMinor == 0 && !input.Visible && input.StockLimit == nil && input.Profile == nil && !input.ActivationRequired && strings.TrimSpace(input.ActivationCode) == "" {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM squad_product_overrides WHERE group_id=?`, uuid); err != nil {
			return model.SquadProduct{}, fmt.Errorf("remove default squad override: %w", err)
		}
		return virtualSquad(input, now), nil
	}
	var profileJSON any
	if input.Profile != nil {
		normalized, normalizeErr := squadprofile.Normalize(input.Profile)
		if normalizeErr != nil {
			return model.SquadProduct{}, normalizeErr
		}
		encoded, marshalErr := json.Marshal(normalized)
		if marshalErr != nil {
			return model.SquadProduct{}, fmt.Errorf("encode squad profile: %w", marshalErr)
		}
		profileJSON = string(encoded)
	}
	hash, err := activationHash(ctx, s.db, uuid, input.ActivationRequired, input.ActivationCode)
	if err != nil {
		return model.SquadProduct{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO squad_product_overrides(group_id,description,profile_json,price_txb_minor,visible,stock_limit,activation_required,activation_code_hash,created_at,updated_at,billing_mode,payg_price_minor,payg_state)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(group_id) DO UPDATE SET description=excluded.description,profile_json=excluded.profile_json,price_txb_minor=excluded.price_txb_minor,visible=excluded.visible,stock_limit=excluded.stock_limit,activation_required=excluded.activation_required,activation_code_hash=excluded.activation_code_hash,updated_at=excluded.updated_at,billing_mode=excluded.billing_mode,payg_price_minor=excluded.payg_price_minor,payg_state=excluded.payg_state`,
		uuid, strings.TrimSpace(input.Description), profileJSON, input.PriceTXBMinor, boolInt(input.Visible), input.StockLimit, boolInt(input.ActivationRequired), hash, stamp(now), stamp(now), input.BillingMode, input.PAYGPriceTXBMinor, input.PAYGState)
	if err != nil {
		return model.SquadProduct{}, fmt.Errorf("save squad override: %w", err)
	}
	product, err := s.SquadProductByGroupID(ctx, uuid)
	if err != nil {
		return model.SquadProduct{}, err
	}
	product.Name = input.Name
	return product, nil
}

func virtualSquad(input SquadProductInput, now time.Time) model.SquadProduct {
	return model.SquadProduct{ID: input.GroupID, GroupID: input.GroupID, Name: input.Name, BillingMode: "combo",
		Description: strings.TrimSpace(input.Description), PriceTXBMinor: input.PriceTXBMinor, Price: model.TXBMoney(input.PriceTXBMinor),
		Profile: input.Profile, Visible: input.Visible, UpstreamPresent: true, StockLimit: input.StockLimit, ActivationRequired: input.ActivationRequired,
		AccessibleNodes: []model.CatalogNode{}, CreatedAt: now, UpdatedAt: now}
}

// SquadProductByGroupID resolves a persisted local override.
func (s *Store) SquadProductByGroupID(ctx context.Context, uuid string) (model.SquadProduct, error) {
	return scanSquad(s.db.QueryRowContext(ctx, squadSelect+` WHERE group_id=?`, uuid))
}

// SquadProductByID treats the public product ID as the upstream UUID.
func (s *Store) SquadProductByID(ctx context.Context, id string) (model.SquadProduct, error) {
	return s.SquadProductByGroupID(ctx, id)
}

// ImportSquad returns the live upstream identity overlaid with local data. It
// deliberately performs no persistence.
func (s *Store) ImportSquad(ctx context.Context, proxySquadUUID, name string) (model.SquadProduct, error) {
	product, err := s.SquadProductByGroupID(ctx, proxySquadUUID)
	if errors.Is(err, ErrNotFound) {
		return virtualSquad(SquadProductInput{GroupID: proxySquadUUID, Name: name, UpstreamPresent: true}, time.Now().UTC()), nil
	}
	if err != nil {
		return model.SquadProduct{}, err
	}
	product.Name = name
	return product, nil
}

// MarkAllSquadsMissing is retained for compatibility and intentionally does
// nothing because presence is now read directly from Proxy.
func (s *Store) MarkAllSquadsMissing(context.Context) error { return nil }

// RefreshImportedSquads is retained for compatibility and intentionally does
// not store upstream-owned identities.
func (s *Store) RefreshImportedSquads(context.Context, []ImportedSquad) error { return nil }

// ListSquadProductOverrides loads merchandising without derived stock counts.
// Compilation, inventory and exports must use this bounded-cost read.
func (s *Store) ListSquadProductOverrides(ctx context.Context, visibleOnly bool) ([]model.SquadProduct, error) {
	query := squadSelect
	if visibleOnly {
		query += ` WHERE visible=1`
	}
	query += ` ORDER BY group_id`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list squad overrides: %w", err)
	}
	defer func() { _ = rows.Close() }()
	products := make([]model.SquadProduct, 0)
	for rows.Next() {
		product, scanErr := scanSquad(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		products = append(products, product)
	}
	return products, rows.Err()
}

func activationHash(ctx context.Context, db *sql.DB, uuid string, required bool, code string) (any, error) {
	if !required {
		return nil, nil
	}
	code = strings.TrimSpace(code)
	if code != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("hash squad activation code: %w", err)
		}
		return string(hash), nil
	}
	var existing sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT activation_code_hash FROM squad_product_overrides WHERE group_id=?`, uuid).Scan(&existing); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("load squad activation code: %w", err)
	}
	if !existing.Valid || existing.String == "" {
		return nil, ErrActivationCodeRequired
	}
	return existing.String, nil
}
