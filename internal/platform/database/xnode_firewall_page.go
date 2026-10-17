package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"go4.org/netipx"
	"net/netip"
	"time"
)

// Pages read a version-bound immutable projection and close rows before any
// network wait. Released generations are retained beyond the command timeout.
func (s *Store) FirewallPage(ctx context.Context, after, target uint64, full bool, cursor string, limit int) (wire.FirewallUpdate, string, error) {
	if limit < 1 || limit > 1000 {
		limit = 256
	}
	result := wire.FirewallUpdate{Version: target}
	rows, err := s.db.QueryContext(ctx, `SELECT key,kind,prefix,generation,until_at,refresh_seconds,released FROM (
 SELECT 'b:'||generation AS key,'ban' AS kind,prefix,generation,until_at,refresh_seconds,released_version AS released FROM xnode_firewall_bans WHERE issued_version<=? AND (released_version=0 OR released_version>?) AND (? OR issued_version>?)
 UNION ALL SELECT 'e:'||id,'exemption',ip,id,until_at,0,released_version FROM xnode_firewall_exemptions WHERE issued_version<=? AND (released_version=0 OR released_version>?) AND (? OR issued_version>?)
 UNION ALL SELECT 'r:'||generation,'remove',prefix,generation,until_at,0,released_version FROM xnode_firewall_bans WHERE ?=0 AND released_version>? AND released_version<=? AND NOT EXISTS(SELECT 1 FROM xnode_firewall_bans newer WHERE newer.prefix=xnode_firewall_bans.prefix AND newer.issued_version<=? AND (newer.released_version=0 OR newer.released_version>?))
 UNION ALL SELECT 'x:'||id,'remove_exemption',ip,id,until_at,0,released_version FROM xnode_firewall_exemptions WHERE ?=0 AND released_version>? AND released_version<=? AND NOT EXISTS(SELECT 1 FROM xnode_firewall_exemptions newer WHERE newer.ip=xnode_firewall_exemptions.ip AND newer.issued_version<=? AND (newer.released_version=0 OR newer.released_version>?))
 ) WHERE key>? ORDER BY key LIMIT ?`, target, target, boolInt(full), after, target, target, boolInt(full), after, boolInt(full), after, target, target, target, boolInt(full), after, target, target, target, cursor, limit)
	if err != nil {
		return result, "", err
	}
	defer rows.Close()
	next := ""
	for rows.Next() {
		var key, kind, prefix, gen, until string
		var seconds int
		var released uint64
		if err = rows.Scan(&key, &kind, &prefix, &gen, &until, &seconds, &released); err != nil {
			return result, "", err
		}
		deadline, e := time.Parse(time.RFC3339Nano, until)
		if e != nil {
			return result, "", e
		}
		switch kind {
		case "ban":
			result.Bans = append(result.Bans, wire.FirewallBan{Prefix: prefix, Generation: gen, Until: deadline, RefreshSeconds: seconds})
		case "exemption":
			result.Exemptions = append(result.Exemptions, wire.FirewallExemption{IP: prefix, Generation: gen, Until: deadline})
		case "remove":
			result.Remove = append(result.Remove, prefix)
		case "remove_exemption":
			result.RemoveExemptions = append(result.RemoveExemptions, prefix)
		}
		next = key
	}
	return result, next, rows.Err()
}
func (s *Store) FirewallBans(ctx context.Context, limit int) ([]xnode.FirewallBanView, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT prefix,generation,ip,rule,detected_count,detected_at,until_at,refresh_seconds,participants,expired FROM xnode_firewall_bans WHERE released_version=0 ORDER BY detected_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []xnode.FirewallBanView{}
	for rows.Next() {
		var v xnode.FirewallBanView
		var at, until string
		var p, e []byte
		if err = rows.Scan(&v.Prefix, &v.Generation, &v.IP, &v.Rule, &v.Count, &at, &until, &v.RefreshSeconds, &p, &e); err != nil {
			return nil, err
		}
		if v.DetectedAt, err = time.Parse(time.RFC3339Nano, at); err != nil {
			return nil, err
		}
		if v.Until, err = time.Parse(time.RFC3339Nano, until); err != nil {
			return nil, err
		}
		participants, err := firewallBitmap(p)
		if err != nil {
			return nil, err
		}
		expired, err := firewallBitmap(e)
		if err != nil {
			return nil, err
		}
		participants.AndNot(expired)
		v.PendingNodes = int(participants.GetCardinality())
		result = append(result, v)
	}
	return result, rows.Err()
}

// FirewallPrefixes projects effective bans while retaining durable audit evidence.
func (s *Store) FirewallPrefixes(ctx context.Context, visit func([]string) error) error {
	var builder netipx.IPSetBuilder
	rows, err := s.db.QueryContext(ctx, `SELECT prefix FROM xnode_firewall_bans WHERE released_version=0`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			break
		}
		prefix, parseErr := netip.ParsePrefix(raw)
		if parseErr != nil {
			err = parseErr
			break
		}
		builder.AddPrefix(prefix)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT ip FROM xnode_firewall_exemptions WHERE released_version=0 AND until_at>?`, stamp(time.Now().UTC()))
	if err != nil {
		return err
	}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			break
		}
		ip, parseErr := netip.ParseAddr(raw)
		if parseErr != nil {
			err = parseErr
			break
		}
		builder.Remove(ip)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	effective, err := builder.IPSet()
	if err != nil {
		return err
	}
	page := make([]string, 0, 512)
	for _, prefix := range effective.Prefixes() {
		page = append(page, prefix.String())
		if len(page) == 512 {
			if err = visit(page); err != nil {
				return err
			}
			page = make([]string, 0, 512)
		}
	}
	if len(page) > 0 {
		return visit(page)
	}
	return nil
}
