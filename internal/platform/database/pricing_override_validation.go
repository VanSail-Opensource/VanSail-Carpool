package database

import (
	"context"
	"database/sql"
)

func validatePricingCommandScope(ctx context.Context, tx *sql.Tx, in PricingIntervention) error {
	allowed := map[string]map[string]bool{
		"FREEZE":  {"global": true, "product": true, "node": true, "machine": true, "model": true},
		"DISABLE": {"product": true}, "MANUAL_RATE": {"product": true}, "PERSONAL_DISCOUNT": {"user_product": true},
		"PAUSE_ADMISSIONS": {"global": true, "product": true, "node": true, "machine": true},
		"MAINTENANCE":      {"node": true, "machine": true}, "SUPPRESS_TELEMETRY": {"global": true, "node": true, "machine": true},
		"RETRY_MODEL": {"product": true, "global": true, "model": true}, "RECALIBRATE": {"product": true, "global": true, "model": true},
		"RESTORE_PRICE": {"product": true}, "DISABLE_EXPERIMENTS": {"global": true, "product": true, "user_product": true},
		"HOLD_RENEWAL": {"purchase": true}, "ACK_INCIDENT": {"incident": true},
	}
	if !allowed[in.Command][in.ScopeType] {
		return ErrConflict
	}
	query := ""
	switch in.ScopeType {
	case "node", "machine":
		query = `SELECT id FROM xnode_nodes WHERE id=?`
	case "purchase":
		query = `SELECT id FROM purchases WHERE id=?`
	case "model":
		query = `SELECT id FROM pricing_model_versions WHERE id=?`
	case "incident":
		query = `SELECT id FROM pricing_operational_incidents WHERE id=?`
	}
	if query != "" {
		var id string
		if err := tx.QueryRowContext(ctx, query, in.ScopeID).Scan(&id); err != nil {
			return err
		}
	}
	return nil
}
