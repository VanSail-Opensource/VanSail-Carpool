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
	case "courtesy_credits", "xnode_daily_usage", "xnode_firewall_exemptions", "activity_draw_results", "activity_draw_revisions", "activity_draw_delivery", "activity_extension_credits", "activity_reward_squad_access", "balances", "ledger_entries", "purchases", "purchase_squad_grants", "payment_orders", "payment_callbacks", "coupon_grants", "coupon_redemptions", "affiliate_commissions", "affiliate_referrals", "audit_events":
		return "financialEvidence"
	case "settings", "sessions", "browser_sessions", "browser_access_links", "api_tokens", "panel_api_tokens", "xnode_identities", "xnode_member_diy", "xnode_routing_plans", "xnode_customizations":
		return "secrets"
	case "database_admin_reviews", "outbox_jobs", "xnode_compile_revision", "xnode_firewall_clock", "xnode_firewall_members", "xnode_firewall_sources", "xnode_firewall_bans":
		return "managedState"
	}
	for _, prefix := range []string{"payment_", "billing_", "purchase_", "provider_operation", "xnode_period", "xnode_traffic", "xnode_payg_", "xnode_imported_", "xnode_accounting", "activity_bet", "activity_raffle_ticket", "activity_lucky_result", "admin_operation", "compensation_"} {
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
