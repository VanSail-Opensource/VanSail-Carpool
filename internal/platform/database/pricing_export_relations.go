package database

import (
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// Related metadata follows retained native ownership or genuine offer relations.
// Definitions are shared; records without an authoritative relation stay excluded.
func pricingRelatedScope(d pricing.Dataset, dimension string, values []string) (string, []any) {
	marks := strings.TrimSuffix(strings.Repeat("?,", len(values)), ",")
	args := []any{}
	for _, v := range values {
		args = append(args, v)
	}
	column := func(v string) string {
		if strings.Contains(v, "COALESCE") {
			return "user_id"
		}
		parts := strings.Split(v, ".")
		return parts[len(parts)-1]
	}
	if dimension == "node" && d.User != "" {
		return `EXISTS(SELECT 1 FROM xnode_daily_usage usage JOIN xnode_identities identity ON identity.proxy_user_id=usage.proxy_user_id LEFT JOIN xnode_payg_accounts payg ON payg.proxy_user_id=usage.proxy_user_id WHERE usage.node_id IN (` + marks + `) AND COALESCE(identity.user_id,payg.user_id)=export_rows.` + column(d.User) + `)`, args
	}
	if dimension == "user" && d.Product != "" {
		query := `EXISTS(SELECT 1 FROM pricing_offers offers WHERE offers.user_id IN (` + marks + `) AND offers.product_id=export_rows.` + column(d.Product) + `) OR EXISTS(SELECT 1 FROM purchases owned WHERE owned.user_id IN (` + marks + `) AND owned.combo_id=export_rows.` + column(d.Product) + `)`
		return "(" + query + ")", append(append([]any(nil), args...), args...)
	}
	if dimension == "node" && d.Product != "" {
		return `EXISTS(SELECT 1 FROM xnode_group_bindings binding WHERE binding.node_id IN (` + marks + `) AND (binding.group_id=export_rows.` + column(d.Product) + ` OR EXISTS(SELECT 1 FROM combos combo JOIN json_each(combo.included_squad_uuids) included ON included.value=binding.group_id WHERE combo.id=export_rows.` + column(d.Product) + `)))`, args
	}
	return "", nil
}
