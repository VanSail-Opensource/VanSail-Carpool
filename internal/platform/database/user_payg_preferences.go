package database

import (
	"context"
	"database/sql"
	"sort"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/preferences"
)

func paygSquadPreferencesFrom(ctx context.Context, reader squadPreferenceReader, user string) (map[string]bool, error) {
	rows, err := reader.QueryContext(ctx, `SELECT group_id FROM user_disabled_payg_squads WHERE user_id=? ORDER BY group_id`, user)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	choices := map[string]bool{}
	for rows.Next() {
		var group string
		if err := rows.Scan(&group); err != nil {
			return nil, err
		}
		choices[group] = false
	}
	return choices, rows.Err()
}

// XNodePAYGPreferences reads only explicit member choices, never admission state.
func (s *Store) XNodePAYGPreferences(ctx context.Context, user string) (preferences.PAYG, error) {
	p, err := s.UserPreferences(ctx, user)
	if err != nil {
		return preferences.PAYG{}, err
	}
	choices, err := paygSquadPreferencesFrom(ctx, s.db, user)
	return preferences.PAYG{Enabled: p.PAYGEnabled, Squads: choices}, err
}

func savePAYGPreferencesTx(ctx context.Context, tx *sql.Tx, user string, patch preferences.Patch, now time.Time) (map[string]bool, error) {
	groups := make([]string, 0, len(patch.PAYGSquads))
	for group := range patch.PAYGSquads {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	for _, group := range groups {
		var valid bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM squad_product_overrides o JOIN xnode_groups g ON g.id=o.group_id WHERE o.group_id=? AND o.billing_mode='payg')`, group).Scan(&valid); err != nil {
			return nil, err
		}
		if !valid {
			return nil, ErrNotFound
		}
		if patch.PAYGSquads[group] {
			if _, err := tx.ExecContext(ctx, `DELETE FROM user_disabled_payg_squads WHERE user_id=? AND group_id=?`, user, group); err != nil {
				return nil, err
			}
		} else if _, err := tx.ExecContext(ctx, `INSERT INTO user_disabled_payg_squads(user_id,group_id) VALUES(?,?) ON CONFLICT(user_id,group_id) DO NOTHING`, user, group); err != nil {
			return nil, err
		}
	}
	if patch.PAYGEnabled != nil || len(groups) > 0 {
		if err := enqueuePAYGUserTx(ctx, tx, user, now); err != nil {
			return nil, err
		}
	}
	return paygSquadPreferencesFrom(ctx, tx, user)
}
