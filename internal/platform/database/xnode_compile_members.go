package database

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// XNodeCompileMembers batches successful reads only for a single compile scope.
// The caller's epoch fence rejects concurrent authoritative input changes.
func (s *Store) XNodeCompileMembers(ctx context.Context, ids []string) (xnode.CompileMembers, error) {
	input := xnode.CompileMembers{Users: map[string]model.User{}, Purchases: map[string][]model.Purchase{}, Periods: map[string]xnode.Period{}, Restricted: map[string]bool{}, Disabled: map[string][]string{}}
	for _, id := range ids {
		input.Purchases[id] = []model.Purchase{}
		input.Disabled[id] = []string{}
		input.Restricted[id] = false
	}
	raw, err := json.Marshal(ids)
	if err != nil {
		return input, err
	}
	if len(ids) == 0 {
		return input, nil
	}
	key := string(raw)
	rows, err := s.db.QueryContext(ctx, userSelect+` WHERE users.id IN (SELECT value FROM json_each(?))`, key)
	if err != nil {
		return input, err
	}
	for rows.Next() {
		user, e := scanUser(rows)
		if e != nil {
			rows.Close()
			return input, e
		}
		input.Users[user.ID] = user
	}
	if err = finishCompileRows(rows); err != nil {
		return input, err
	}
	rows, err = s.db.QueryContext(ctx, purchaseSelect+` WHERE purchases.user_id IN (SELECT value FROM json_each(?)) ORDER BY purchases.valid_from DESC,purchases.created_at DESC`, key)
	if err != nil {
		return input, err
	}
	for rows.Next() {
		purchase, e := scanPurchase(rows)
		if e != nil {
			rows.Close()
			return input, e
		}
		input.Purchases[purchase.UserID] = append(input.Purchases[purchase.UserID], purchase)
	}
	if err = finishCompileRows(rows); err != nil {
		return input, err
	}
	groups := map[string][]string{}
	rows, err = s.db.QueryContext(ctx, `WITH selected_purchases AS (SELECT `+purchaseSquadColumns+` FROM purchases WHERE user_id IN (SELECT value FROM json_each(?))), `+purchaseSquadMembershipCTE+` SELECT DISTINCT purchase_id,group_id FROM owned_squads ORDER BY purchase_id,group_id`, key)
	if err != nil {
		return input, err
	}
	for rows.Next() {
		var purchase, group string
		if err = rows.Scan(&purchase, &group); err != nil {
			rows.Close()
			return input, err
		}
		groups[purchase] = append(groups[purchase], group)
	}
	if err = finishCompileRows(rows); err != nil {
		return input, err
	}
	for id, purchases := range input.Purchases {
		for i := range purchases {
			purchases[i].SquadUUIDs = groups[purchases[i].ID]
		}
		input.Purchases[id] = purchases
	}
	rows, err = s.db.QueryContext(ctx, `SELECT `+xnodePeriodColumns+` FROM xnode_periods WHERE proxy_user_id IN (SELECT proxy_user_id FROM xnode_identities WHERE user_id IN (SELECT value FROM json_each(?)))`, key)
	if err != nil {
		return input, err
	}
	for rows.Next() {
		period, e := scanXNodePeriod(rows)
		if e != nil {
			rows.Close()
			return input, e
		}
		input.Periods[period.ID] = period
	}
	if err = finishCompileRows(rows); err != nil {
		return input, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT user_id,squad_uuid FROM user_disabled_squads WHERE user_id IN (SELECT value FROM json_each(?)) ORDER BY user_id,squad_uuid`, key)
	if err != nil {
		return input, err
	}
	for rows.Next() {
		var id, squad string
		if err = rows.Scan(&id, &squad); err != nil {
			rows.Close()
			return input, err
		}
		input.Disabled[id] = append(input.Disabled[id], squad)
	}
	if err = finishCompileRows(rows); err != nil {
		return input, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT id FROM users WHERE id IN (SELECT value FROM json_each(?)) AND (EXISTS(SELECT 1 FROM admin_temporary_bans WHERE user_id=users.id AND restored_at IS NULL) OR EXISTS(SELECT 1 FROM xnode_identities WHERE user_id=users.id AND access_disabled=1))`, key)
	if err != nil {
		return input, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return input, err
		}
		input.Restricted[id] = true
	}
	return input, finishCompileRows(rows)
}

func finishCompileRows(rows interface {
	Err() error
	Close() error
}) error {
	err := rows.Err()
	closeErr := rows.Close()
	if err != nil {
		return err
	}
	return closeErr
}
