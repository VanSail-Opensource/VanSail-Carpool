package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

// PricingExportScope describes retained, authorized data rather than reconstructed history.
type PricingExportScope struct {
	From             time.Time `json:"from"`
	Until            time.Time `json:"until"`
	Users            []string  `json:"users"`
	Nodes            []string  `json:"nodes"`
	Products         []string  `json:"products"`
	Category         string    `json:"category"`
	Format           string    `json:"format"`
	IncludeUsers     bool      `json:"includeUsers"`
	IncludeFinancial bool      `json:"includeFinancial"`
	Pseudonymize     bool      `json:"pseudonymize"`
}

func (s PricingExportScope) Validate() error {
	if s.Format != "csv" && s.Format != "jsonl" {
		return ErrConflict
	}
	if s.Category != "" && s.Category != "raw" && s.Category != "features" && s.Category != "forecasts" && s.Category != "pricing" && s.Category != "operations" && s.Category != "validation" {
		return ErrConflict
	}
	if !s.From.IsZero() && !s.Until.IsZero() && !s.Until.After(s.From) {
		return ErrConflict
	}
	for _, ids := range [][]string{s.Users, s.Nodes, s.Products} {
		if len(ids) > 100 {
			return ErrConflict
		}
		for _, id := range ids {
			if strings.TrimSpace(id) == "" || len(id) > 128 {
				return ErrConflict
			}
		}
	}
	return nil
}

func pricingDatasetQuery(d pricing.Dataset, s PricingExportScope) (string, []any) {
	conditions := []string{}
	if d.Name == "features/features" {
		conditions = append(conditions, "export_rows.entity_type NOT IN ('compute_input','compute_result')")
	}
	args := []any{}
	column := func(value string) string {
		if strings.Contains(value, "COALESCE") {
			return "export_rows.user_id"
		}
		parts := strings.Split(value, ".")
		if value == "" {
			return ""
		}
		return "export_rows." + parts[len(parts)-1]
	}
	addIDs := func(expr string, values []string, dimension string) {
		if len(values) == 0 {
			return
		}
		if expr == "" {
			if related, bindings := pricingRelatedScope(d, dimension, values); related != "" {
				conditions = append(conditions, related)
				args = append(args, bindings...)
				return
			}
			if !d.Shared {
				conditions = append(conditions, "1=0")
			}
			return
		}
		marks := make([]string, len(values))
		for i, v := range values {
			marks[i] = "?"
			args = append(args, v)
		}
		conditions = append(conditions, expr+" IN ("+strings.Join(marks, ",")+")")
	}
	entity := d.Name == "features/features" || d.Name == "forecasts/historical_predictions" || d.Name == "forecasts/prediction_evaluations"
	prefix := "export_rows."
	if d.Name == "forecasts/prediction_evaluations" {
		prefix = "export_rows."
	}
	if entity {
		if !s.IncludeUsers {
			conditions = append(conditions, prefix+"entity_type NOT IN ('user','user_node')")
		}
		if len(s.Nodes) > 0 {
			parts := []string{}
			for _, node := range s.Nodes {
				parts = append(parts, "("+prefix+"entity_type='node' AND "+prefix+"entity_id=? OR "+prefix+"entity_type='user_node' AND SUBSTR("+prefix+"entity_id,LENGTH("+prefix+"entity_id)-LENGTH(?))=':'||?)")
				args = append(args, node, node, node)
			}
			conditions = append(conditions, "("+strings.Join(parts, " OR ")+")")
		}
		if len(s.Users) > 0 {
			parts := []string{}
			for _, user := range s.Users {
				parts = append(parts, "("+prefix+"entity_type='user' AND "+prefix+"entity_id=? OR "+prefix+"entity_type='user_node' AND SUBSTR("+prefix+"entity_id,1,LENGTH(?)+1)=?||':')")
				args = append(args, user, user, user)
			}
			conditions = append(conditions, "("+strings.Join(parts, " OR ")+")")
		}
		if len(s.Products) > 0 {
			conditions = append(conditions, prefix+"entity_type='product'")
			products := []string{}
			for _, id := range s.Products {
				if strings.Contains(id, ":") {
					products = append(products, id)
				} else {
					for _, kind := range []string{"combo", "payg", "addon"} {
						products = append(products, kind+":"+id)
					}
				}
			}
			addIDs(prefix+"entity_id", products, "product")
		}
	} else {
		addIDs(column(d.User), s.Users, "user")
		addIDs(column(d.Node), s.Nodes, "node")
		addIDs(column(d.Product), s.Products, "product")
	}
	if d.Date != "" && !d.Shared {
		var from, until any = pricingStamp(s.From), pricingStamp(s.Until)
		if strings.HasSuffix(d.Date, "_ns") {
			from, until = s.From.UnixNano(), s.Until.UnixNano()
		}
		if d.Date == "day" || d.Date == "d.date" {
			from = s.From.UTC().Format("2006-01-02")
			until = s.Until.UTC().Format("2006-01-02")
		}
		if !s.From.IsZero() {
			conditions = append(conditions, column(d.Date)+">=?")
			args = append(args, from)
		}
		if !s.Until.IsZero() {
			conditions = append(conditions, column(d.Date)+"<?")
			args = append(args, until)
		}
	}
	query := "SELECT * FROM (" + d.Query + ") export_rows"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	return query, args
}

func pricingDatasetIncluded(d pricing.Dataset, s PricingExportScope) bool {
	if d.Financial && !s.IncludeFinancial {
		return false
	}
	if d.Users && !s.IncludeUsers && !strings.HasPrefix(d.Name, "features/") && !strings.HasPrefix(d.Name, "forecasts/historical") && d.Name != "forecasts/prediction_evaluations" {
		return false
	}
	return s.Category == "" || strings.HasPrefix(d.Name, s.Category+"/")
}

func pricingExportName(d pricing.Dataset, format string) string {
	return fmt.Sprintf("bapo-export/%s.%s.gz", d.Name, format)
}
