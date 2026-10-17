package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"time"
)

func (s *Store) PurchaseFirewallExemption(ctx context.Context, user string, input xnode.FirewallPurchase, price int64, now time.Time) (xnode.FirewallGrant, error) {
	var grant xnode.FirewallGrant
	err := s.firewallWrite(ctx, func(tx *sql.Tx) error {
		var until string
		err := tx.QueryRowContext(ctx, `SELECT id,ip,until_at,price_minor,hours FROM xnode_firewall_exemptions WHERE user_id=? AND idempotency_key=?`, user, input.Key).Scan(&grant.ID, &grant.IP, &until, &grant.PriceMinor, &grant.Hours)
		if err == nil {
			if grant.IP != input.IP || grant.PriceMinor != price || grant.Hours != input.Hours {
				return xnode.ErrConflict
			}
			grant.Until, err = time.Parse(time.RFC3339Nano, until)
			return err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var existing int
		if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_firewall_exemptions WHERE ip=? AND until_at>? AND released_version=0`, input.IP, stamp(now)).Scan(&existing); err != nil {
			return err
		}
		if existing > 0 {
			return xnode.ErrConflict
		}
		balance, err := changeBalanceTx(ctx, tx, user, -price, now)
		if err != nil {
			return err
		}
		id, err := ids.New()
		if err != nil {
			return err
		}
		ledger, err := insertLedgerTx(ctx, tx, user, -price, balance, "firewall_exemption", id, "", now)
		if err != nil {
			return err
		}
		version, err := firewallVersion(ctx, tx)
		if err != nil {
			return err
		}
		grant = xnode.FirewallGrant{ID: id, IP: input.IP, PriceMinor: price, Hours: input.Hours, Until: now.Add(time.Duration(input.Hours) * time.Hour)}
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_firewall_exemptions(id,user_id,idempotency_key,ip,price_minor,hours,until_at,created_at,issued_version,ledger_id) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, user, input.Key, input.IP, price, input.Hours, stamp(grant.Until), stamp(now), version, ledger); err != nil {
			return err
		}
		return queueFirewallFleetTx(ctx, tx, now)
	})
	return grant, err
}
