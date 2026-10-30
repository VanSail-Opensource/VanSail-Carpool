package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"slices"
	"time"
)

func removeGroupFromCombosTx(ctx context.Context, tx *sql.Tx, id string, now time.Time) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,included_squad_uuids FROM combos WHERE EXISTS(SELECT 1 FROM json_each(combos.included_squad_uuids) j WHERE j.value=?)`, id)
	if err != nil {
		return err
	}
	type combo struct{ id, raw string }
	items := []combo{}
	for rows.Next() {
		var c combo
		if err = rows.Scan(&c.id, &c.raw); err != nil {
			rows.Close()
			return err
		}
		items = append(items, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, c := range items {
		var groups []string
		if err = json.Unmarshal([]byte(c.raw), &groups); err != nil {
			return err
		}
		// Freeze the existing legacy fallback before changing its live source.
		if _, err = tx.ExecContext(ctx, `UPDATE purchases SET combo_squad_uuids=? WHERE combo_id=? AND combo_squad_uuids IS NULL`, c.raw, c.id); err != nil {
			return err
		}
		groups = slices.DeleteFunc(groups, func(group string) bool { return group == id })
		if groups == nil {
			groups = []string{}
		}
		raw, err := json.Marshal(groups)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE combos SET included_squad_uuids=?,updated_at=? WHERE id=?`, string(raw), stamp(now), c.id); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM pricing_personal_decisions WHERE product_type='combo' AND product_id=?`, c.id); err != nil {
			return err
		}
	}
	return nil
}
