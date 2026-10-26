package database

import (
	"context"

	"encoding/json"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PricingIntervention is a bounded domain command with an optimistic precondition.
type PricingIntervention struct {
	ID              string    `json:"id"`
	ScopeType       string    `json:"scopeType"`
	ScopeID         string    `json:"scopeId"`
	Command         string    `json:"command"`
	Reason          string    `json:"reason"`
	Price           int64     `json:"priceMinor,string"`
	DiscountBPS     int64     `json:"discountBps"`
	Budget          int64     `json:"budgetMinor,string"`
	PriceVersion    string    `json:"priceVersion"`
	ExpectedVersion int64     `json:"expectedVersion"`
	ExpiresAt       time.Time `json:"expiresAt"`
	Status          string    `json:"status"`
}

var pricingCommands = map[string]bool{"FREEZE": true, "DISABLE": true, "MANUAL_RATE": true, "PERSONAL_DISCOUNT": true, "PAUSE_ADMISSIONS": true, "MAINTENANCE": true, "SUPPRESS_TELEMETRY": true, "RETRY_MODEL": true, "RECALIBRATE": true, "RESTORE_PRICE": true, "DISABLE_EXPERIMENTS": true, "HOLD_RENEWAL": true, "ACK_INCIDENT": true}

// CreatePricingIntervention validates, versions and audits a command atomically.
func (s *Store) retryAttempt_CreatePricingIntervention(ctx context.Context, actor string, in PricingIntervention, now time.Time) (PricingIntervention, error) {
	if !pricingCommands[in.Command] || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 1000 || in.ScopeID == "" || len(in.ScopeID) > 256 || !in.ExpiresAt.After(now) || in.ExpiresAt.After(now.Add(7*24*time.Hour)) {
		return in, ErrConflict
	}
	if in.ScopeType != "global" && in.ScopeType != "product" && in.ScopeType != "node" && in.ScopeType != "machine" && in.ScopeType != "user_product" && in.ScopeType != "purchase" && in.ScopeType != "model" && in.ScopeType != "incident" {
		return in, ErrConflict
	}
	if (in.Command == "MANUAL_RATE" || in.Command == "DISABLE" || in.Command == "RESTORE_PRICE") && in.ScopeType != "product" || in.Command == "PERSONAL_DISCOUNT" && in.ScopeType != "user_product" || in.Command == "HOLD_RENEWAL" && in.ScopeType != "purchase" {
		return in, ErrConflict
	}
	if in.Command == "PERSONAL_DISCOUNT" && (in.DiscountBPS <= 0 || in.DiscountBPS > 10000 || in.Budget <= 0 || in.Budget > pricing.MaxPrice) {
		return in, ErrConflict
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return in, err
	}
	defer tx.Rollback()
	var serial int
	if err = tx.QueryRowContext(ctx, `SELECT singleton FROM pricing_worker_watermark WHERE singleton=1 /*pg:FOR UPDATE*/`).Scan(&serial); err != nil {
		return in, err
	}
	if err = pricingAdminTx(ctx, tx, actor); err != nil {
		return in, err
	}
	if in.Command == "PERSONAL_DISCOUNT" {
		parts := strings.SplitN(in.ScopeID, ":", 3)
		if len(parts) != 3 {
			return in, ErrConflict
		}
		var user string
		if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=?`, parts[0]).Scan(&user); err != nil {
			return in, err
		}
		fixed, e := pricingReference(ctx, tx, parts[1], parts[2])
		if e != nil {
			return in, e
		}
		p, e := loadPricingPolicy(ctx, tx, parts[1], parts[2], fixed)
		if e != nil {
			return in, e
		}
		if !p.DynamicEnabled || !p.PersonalDiscounts {
			return in, ErrConflict
		}
	}
	if err = validatePricingCommandScope(ctx, tx, in); err != nil {
		return in, err
	}
	previous := "{}"
	if in.ScopeType == "product" {
		parts := strings.SplitN(in.ScopeID, ":", 2)
		if len(parts) != 2 {
			return in, ErrConflict
		}
		p, e := loadPricingPolicy(ctx, tx, parts[0], parts[1], 0)
		if e != nil {
			return in, e
		}
		if p.Version == 0 || p.Version != in.ExpectedVersion {
			return in, pricing.VersionConflict
		}
		if in.Command == "MANUAL_RATE" {
			if in.Price < 0 || in.Price > pricing.MaxPrice || p.ProductType == "payg" && in.Price == 0 {
				return in, ErrConflict
			}
		}
		raw, e := json.Marshal(p)
		if e != nil {
			return in, e
		}
		previous = string(raw)
	} else {
		var version int64
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(expected_version),0) FROM pricing_admin_overrides WHERE scope_type=? AND scope_id=?`, in.ScopeType, in.ScopeID).Scan(&version); err != nil {
			return in, err
		}
		if version != in.ExpectedVersion {
			return in, pricing.VersionConflict
		}
	}
	in.ID, err = ids.New()
	if err != nil {
		return in, err
	}
	in.Status = "ACTIVE"
	if in.ScopeType != "product" {
		in.ExpectedVersion++
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return in, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO pricing_admin_overrides(id,scope_type,scope_id,command,reason,created_by,starts_at,expires_at,expected_version,previous_json,proposed_json,status,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, in.ID, in.ScopeType, in.ScopeID, in.Command, in.Reason, actor, pricingStamp(now), pricingStamp(in.ExpiresAt), in.ExpectedVersion, previous, string(raw), in.Status, pricingStamp(now))
	if err != nil {
		return in, err
	}
	if in.Status == "ACTIVE" {
		if err = activatePricingOverrideTx(ctx, tx, actor, &in, now); err != nil {
			return in, err
		}
	}
	if err = pricingAuditTx(ctx, tx, actor, "intervention.create", in.ID, in.Reason, in, now); err != nil {
		return in, err
	}
	if err = pricingChangeNoticeTx(ctx, tx, actor, in.Command, in.ScopeID, in.Reason, now); err != nil {
		return in, err
	}
	if err = tx.Commit(); err == nil {
		s.pricingCache.Clear()
		s.pricingExposureCache.Clear()
	}
	return in, err
}
