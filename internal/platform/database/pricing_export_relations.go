package database

import (
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// Related metadata follows retained native ownership or genuine offer relations.
// Definitions are shared; records without an authoritative relation stay excluded.
func pricingRelatedScope(d pricing.Dataset, dimension string, values []string, first ...int) (string, []any) {
	start := 1
	if len(first) > 0 {
		start = first[0]
	}
	marks := placeholders(len(values), start)
	secondMarks := placeholders(len(values), start+len(values))
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
		kind := pricingDatasetProductType(d)
		query := `EXISTS(SELECT 1 FROM pricing_offers offers WHERE offers.user_id IN (` + marks + `) AND offers.product_type=` + kind + ` AND offers.product_id=export_rows.` + column(d.Product) + `) OR EXISTS(SELECT 1 FROM purchases owned WHERE owned.user_id IN (` + secondMarks + `) AND ` + kind + `='combo' AND owned.combo_id=export_rows.` + column(d.Product) + `)`
		return "(" + query + ")", append(append([]any(nil), args...), args...)
	}
	if dimension == "node" && d.Product != "" {
		kind := pricingDatasetProductType(d)
		return `EXISTS(SELECT 1 FROM xnode_group_bindings binding WHERE binding.node_id IN (` + marks + `) AND (` + kind + ` IN ('addon','payg') AND binding.group_id=export_rows.` + column(d.Product) + ` OR ` + kind + `='combo' AND EXISTS(SELECT 1 FROM combos combo JOIN jsonb_array_elements_text((combo.included_squad_uuids)::jsonb) included ON included.value=binding.group_id WHERE combo.id=export_rows.` + column(d.Product) + `)))`, args
	}
	return "", nil
}

func pricingDatasetProductType(d pricing.Dataset) string {
	if strings.Contains(d.Query, "product_type") {
		return "export_rows.product_type"
	}
	if d.Name == "raw/payg_charges" {
		return "'payg'"
	}
	return "'combo'"
}
