package database

import (
	"context"
	"database/sql"
	"net/netip"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// Only exact bans attributed to leased addresses are released. A containing
// IPv6 /64 or other range remains restricted for every neighboring address.
// Existing ban/audit evidence is retained; lease addresses are never inserted.
func (s *Store) ReleaseAutomaticLeaseBans(ctx context.Context, items []xnode.AutomaticLeaseGrant, now time.Time) error {
	if len(items) == 0 {
		return nil
	}
	if len(items) > wire.MaxAutomaticLeaseBatch {
		return xnode.ErrInvalid
	}
	var ips, prefixes, deadlines []string
	for _, item := range items {
		ip, err := wire.AutomaticLeaseAddress(item.IP)
		if err != nil || item.Until.IsZero() {
			return xnode.ErrInvalid
		}
		ips = append(ips, ip.String())
		prefixes = append(prefixes, netip.PrefixFrom(ip, ip.BitLen()).String())
		deadlines = append(deadlines, stamp(item.Until))
	}
	return s.firewallWrite(ctx, func(tx *sql.Tx) error {
		asOf := now
		if wall := time.Now().UTC(); wall.After(asOf) {
			asOf = wall
		}
		var found bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM xnode_firewall_bans b JOIN unnest($1::text[],$2::text[],$3::timestamptz[]) AS l(ip,prefix,until_at) ON b.ip=l.ip AND b.prefix=l.prefix WHERE b.released_version=0 AND l.until_at>$4::timestamptz)`, ips, prefixes, deadlines, stamp(asOf)).Scan(&found); err != nil {
			return err
		}
		if !found {
			return nil
		}
		version, err := firewallVersion(ctx, tx)
		if err != nil {
			return err
		}
		// The clock lock can wait behind a concurrent firewall transaction.
		// Never release a ban for a lease that expired during that wait.
		if wall := time.Now().UTC(); wall.After(asOf) {
			asOf = wall
		}
		if _, err := tx.ExecContext(ctx, `UPDATE xnode_firewall_bans b SET released_version=$1,released_at=$2 FROM unnest($3::text[],$4::text[],$5::timestamptz[]) AS l(ip,prefix,until_at) WHERE b.released_version=0 AND b.ip=l.ip AND b.prefix=l.prefix AND l.until_at>$6::timestamptz`, version, stamp(now), ips, prefixes, deadlines, stamp(asOf)); err != nil {
			return err
		}
		return queueFirewallFleetTx(ctx, tx, now)
	})
}
