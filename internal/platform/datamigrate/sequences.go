package datamigrate

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

func readSequences(ctx context.Context, q queryer, pg bool) (map[string]int64, error) {
	result := map[string]int64{}
	for table, column := range map[string]string{"xnode_identities": "proxy_user_id", "xnode_firewall_members": "ordinal"} {
		var value int64
		if !pg {
			err := q.QueryRowContext(ctx, `SELECT seq FROM sqlite_sequence WHERE name=?`, table).Scan(&value)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		} else {
			var name string
			if err := q.QueryRowContext(ctx, `SELECT pg_get_serial_sequence(?,?)`, table, column).Scan(&name); err != nil {
				return nil, err
			}
			parts := strings.Split(name, ".")
			for i := range parts {
				parts[i] = quote(parts[i])
			}
			var called bool
			if err := q.QueryRowContext(ctx, `SELECT last_value,is_called FROM `+strings.Join(parts, ".")).Scan(&value, &called); err != nil {
				return nil, err
			}
			if !called {
				value = 0
			}
		}
		result[table] = value
	}
	// The service sequence is deliberately separate from the member identity
	// default. Cover its high water mark in recovery evidence as well.
	service := result["xnode_identities"]
	if service < 4503599627370496 {
		service = 0
	}
	if pg {
		var called bool
		if err := q.QueryRowContext(ctx, `SELECT last_value,is_called FROM xnode_service_proxy_id_seq`).Scan(&service, &called); err != nil {
			return nil, err
		}
		if !called {
			service = 0
		}
	}
	result["xnode_service_proxy_id_seq"] = service
	return result, nil
}
