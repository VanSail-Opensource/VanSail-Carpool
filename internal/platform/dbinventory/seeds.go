package dbinventory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var fixtureTimestamp = regexp.MustCompile(`'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z'`)

// WriteFixtureSeeds exports only bootstrap rows from the newly created private
// fixture. Callers must never invoke this against a production/source database.
func WriteFixtureSeeds(ctx context.Context, db *sql.DB, output string) error {
	var script strings.Builder
	script.WriteString("-- Synthetic empty-installation bootstrap rows only.\n")
	for _, table := range []string{"affiliate_config_versions", "affiliate_tiers", "affiliate_config_current", "node_compensation_config", "onboarding_content", "payment_profiles", "txb_limits", "xnode_compile_revision", "xnode_firewall_clock"} {
		rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?) ORDER BY cid`, table)
		if err != nil {
			return err
		}
		var columns, expressions []string
		for rows.Next() {
			var column string
			if err = rows.Scan(&column); err != nil {
				rows.Close()
				return err
			}
			quoted := `"` + strings.ReplaceAll(column, `"`, `""`) + `"`
			columns = append(columns, quoted)
			expressions = append(expressions, "quote("+quoted+")")
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		rows, err = db.QueryContext(ctx, `SELECT `+strings.Join(expressions, `||','||`)+` FROM "`+table+`" ORDER BY rowid`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var values string
			if err = rows.Scan(&values); err != nil {
				rows.Close()
				return err
			}
			values = fixtureTimestamp.ReplaceAllString(values, "'2000-01-01T00:00:00Z'")
			fmt.Fprintf(&script, "INSERT INTO \"%s\"(%s) VALUES(%s);\n", table, strings.Join(columns, ","), values)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(output, 0o750); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "fixture-seeds.sql"), []byte(script.String()), 0o600)
}
