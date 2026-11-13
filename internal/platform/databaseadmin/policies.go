package databaseadmin

import (
	"fmt"
	"strings"
)

// ProtectionError gives the same stable reason as catalog metadata without
// returning SQL, values, credentials or database exception details.
type ProtectionError struct{ Table, Column, Reason string }

func (e *ProtectionError) Error() string {
	return fmt.Sprintf("%s.%s: %s", e.Table, e.Column, e.Reason)
}
func (e *ProtectionError) Unwrap() error { return ErrInvalidValue }

func tableProtection(name string) string {
	switch name {
	case "refunds", "renewal_batches", "coupon_uses", "coupon_grant_discards", "affiliate_tier_awards", "affiliate_config_versions", "abuse_incident_facts", "ip_lookup_allowances", "ip_lookup_checks", "ip_lookup_reports", "xnode_node_rates", "xnode_stream_rates", "xnode_report_cursors", "xnode_metered_probes", "node_compensation_events", "node_compensation_event_recipients", "node_compensation_event_squads", "courtesy_credits", "xnode_daily_usage", "xnode_firewall_exemptions", "activity_draw_results", "activity_draw_revisions", "activity_draw_delivery", "activity_extension_credits", "activity_reward_squad_access", "balances", "ledger_entries", "purchases", "purchase_squad_grants", "payment_orders", "payment_callbacks", "coupon_grants", "coupon_redemptions", "affiliate_commissions", "affiliate_referrals", "audit_events":
		return "financialEvidence"
	case "settings", "sessions", "browser_sessions", "browser_access_links", "api_tokens", "panel_api_tokens", "xnode_identities", "xnode_member_diy", "xnode_routing_plans", "xnode_customizations":
		return "secrets"
	case "backup_runs", "backup_uploads", "restore_jobs", "maintenance_runs", "pricing_worker_watermark", "statistics_snapshots", "connection_ip_blocks", "connection_scans", "database_admin_reviews", "outbox_jobs", "telegram_star_reconciliation", "xnode_compile_revision", "xnode_firewall_clock", "xnode_firewall_members", "xnode_firewall_sources", "xnode_firewall_bans":
		return "managedState"
	}
	if strings.HasPrefix(name, "pricing_") && name != "pricing_product_settings" && name != "pricing_admin_overrides" && name != "pricing_operational_incidents" {
		return "financialEvidence"
	}
	if strings.HasPrefix(name, "activity_") && name != "activity_games" && name != "activity_lucky_prizes" {
		return "financialEvidence"
	}
	if strings.HasPrefix(name, "hourly_") {
		return "financialEvidence"
	}
	if strings.HasPrefix(name, "account_cleanup_") || strings.HasPrefix(name, "xnode_usage_") {
		return "managedState"
	}
	for _, prefix := range []string{"payment_", "usdt_", "billing_", "purchase_", "provider_operation", "xnode_period", "xnode_traffic", "xnode_payg_", "xnode_imported_", "xnode_accounting", "activity_bet", "activity_raffle_ticket", "activity_lucky_result", "admin_operation", "compensation_"} {
		if strings.HasPrefix(name, prefix) {
			return "financialEvidence"
		}
	}
	for _, part := range []string{"_ledger", "_audit", "_receipt", "_settlement", "_refund", "_redemption", "_credential", "_session", "_tombstone"} {
		if strings.Contains(name, part) {
			return "financialEvidence"
		}
	}
	if strings.Contains(name, "token") || strings.Contains(name, "secret") {
		return "secrets"
	}
	return ""
}

func generatedColumn(column schemaColumn) bool {
	if column.Hidden != 0 {
		return true
	}
	if column.Default.Valid && strings.Contains(strings.ToLower(column.Default.String), "nextval(") {
		return true
	}
	switch column.Name {
	case "created_at", "updated_at", "first_synced_at", "retired_at", "desired_revision", "accepted_revision", "applied_revision", "member_count":
		return true
	}
	return false
}

func columnProtection(table string, column schemaColumn, insert bool) string {
	if reason := tableProtection(table); reason != "" {
		return reason
	}
	if column.Sensitive {
		return "secrets"
	}
	if generatedColumn(column) {
		return "generated"
	}
	if !insert && column.PrimaryKeyPosition > 0 {
		return "primaryKey"
	}
	return ""
}

func allowMutation(schema tableSchema, action string) error {
	allowed := action == "insert" && schema.AllowInsert || action == "update" && schema.AllowUpdate || action == "delete" && schema.AllowDelete
	if !allowed {
		reason := schema.ProtectionReason
		if reason == "" {
			reason = "protectedDependencies"
		}
		return &ProtectionError{Table: schema.Name, Reason: reason}
	}
	return nil
}
