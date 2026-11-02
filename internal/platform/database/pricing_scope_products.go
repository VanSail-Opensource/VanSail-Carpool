package database

import (
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
	"strings"
)

// Typed keys preserve product identity even when a Combo and squad share an ID.
// Bare legacy IDs keep their documented cross-type matching behavior.
func pricingTypedProductScope(d pricing.Dataset, values []string, first ...int) (string, []any) {
	start := 1
	if len(first) > 0 {
		start = first[0]
	}
	conditions := []string{}
	args := []any{}
	for _, value := range values {
		parts := strings.SplitN(value, ":", 2)
		kind, id := "", value
		if len(parts) == 2 {
			kind, id = parts[0], parts[1]
			if kind != "combo" && kind != "addon" && kind != "payg" {
				continue
			}
		}
		if d.Product != "" {
			column := strings.Split(d.Product, ".")
			columnName := column[len(column)-1]
			if kind != "" && strings.Contains(d.Query, "product_type") {
				conditions = append(conditions, fmt.Sprintf("(export_rows.product_type=$%d AND export_rows.", start+len(args))+
					columnName+fmt.Sprintf("=$%d)", start+len(args)+1),
				)
				args = append(args, kind, id)
			} else {
				expected := "combo"
				if d.Name == "raw/payg_charges" {
					expected = "payg"
				}
				if kind != "" && kind != expected {
					continue
				}
				conditions = append(conditions, "export_rows."+columnName+fmt.Sprintf("=$%d", start+len(args)))
				args = append(args, id)
			}
		} else if d.Name == "operations/admin_overrides" {
			typed := value
			if kind == "" {
				conditions = append(conditions, "export_rows.scope_type='global'")
				continue
			}
			conditions = append(conditions, fmt.Sprintf("(export_rows.scope_type='global' OR export_rows.scope_type='product' AND export_rows.scope_id=$%d OR export_rows.scope_type='user_product' AND SUBSTR(export_rows.scope_id,LENGTH(export_rows.scope_id)-LENGTH($%d))=':'||$%d)", start+len(args), start+len(args)+1, start+len(args)+2))
			args = append(args, typed, typed, typed)
		} else if d.Name == "forecasts/model_versions" || strings.HasPrefix(d.Name, "validation/") {
			if kind == "" {
				// These definitions were shared for legacy untyped export filters.
				if d.Shared {
					return "", nil
				}
				continue
			}
			modelQuery := fmt.Sprintf("(m.kind=$%d", start+len(args))
			bindings := []any{"response:" + kind + ":" + id}
			if kind == "combo" {
				modelQuery += fmt.Sprintf(" OR m.kind=$%d", start+len(args)+1)
				bindings = append(bindings, "renewal:"+id)
			}
			modelQuery += ")"
			if d.Name == "forecasts/model_versions" {
				conditions = append(conditions, strings.ReplaceAll(modelQuery, "m.", "export_rows."))
			} else {
				conditions = append(conditions, "export_rows.model_version IN (SELECT m.id FROM pricing_model_versions m WHERE "+modelQuery+")")
			}
			args = append(args, bindings...)
		} else if d.Shared {
			return "", nil
		}
	}
	if len(conditions) == 0 {
		return "1=0", nil
	}
	return "(" + strings.Join(conditions, " OR ") + ")", args
}
