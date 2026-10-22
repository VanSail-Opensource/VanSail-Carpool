package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/RoaringBitmap/roaring/v2"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"strconv"
	"strings"
	"time"
)

func (s *Store) RecordFirewallEvents(ctx context.Context, node, namespace string, events []wire.FirewallEvent, policy wire.FirewallPolicy, now time.Time, excluded ...[]string) error {
	if !wire.ValidName(namespace) || len(events) > 256 {
		return xnode.ErrInvalid
	}
	s.firewallDetectionMu.Lock()
	defer s.firewallDetectionMu.Unlock()
	if s.firewallWindows == nil {
		s.firewallWindows = map[string]firewallDetectionWindow{}
	}
	staged := map[string]firewallDetectionWindow{}
	ignored := map[string]bool{}
	for _, list := range excluded {
		for _, ip := range list {
			ignored[ip] = true
		}
	}
	result := s.firewallWrite(ctx, func(tx *sql.Tx) error {
		var ordinal uint32
		if err := tx.QueryRowContext(ctx, `SELECT ordinal FROM xnode_firewall_members WHERE node_id=? AND active=1`, node).Scan(&ordinal); err != nil {
			return err
		}
		var cursor uint64
		if err := tx.QueryRowContext(ctx, `SELECT sequence FROM xnode_firewall_cursors WHERE node_id=? AND namespace=?`, node, namespace).Scan(&cursor); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		for _, event := range events {
			raw, ok := strings.CutPrefix(event.ID, namespace+"-")
			sequence, err := strconv.ParseUint(raw, 10, 63)
			if !ok || err != nil || len(raw) != 20 || sequence == 0 {
				return xnode.ErrInvalid
			}
			if sequence <= cursor {
				continue
			}
			switch event.Kind {
			case "detected":
				rule, enabled := policy.Rule(event.Rule)
				if enabled && policy.Protect {
					ip, e := xnode.FirewallPublicIP(event.IP)
					if e != nil {
						return e
					}
					if !ignored[ip] {
						event.IP = ip
						aggregated, ban, e := s.stageDetection(event, rule, policy, now, staged)
						if e != nil {
							return e
						}
						if ban {
							prefix, e := wire.FirewallPrefix(ip)
							if e != nil {
								return e
							}
							if err = authorizeDetectionTx(ctx, tx, prefix.String(), ip, aggregated, policy, now); err != nil {
								return err
							}
						}
					}
				}
			case "expired":
				if err = expireFirewallParticipant(ctx, tx, "xnode_firewall_bans", event.Prefix, event.Generation, ordinal); err != nil {
					return err
				}
			case "source_expired":
				if err = expireFirewallParticipant(ctx, tx, "xnode_firewall_sources", event.Rule, event.Generation, ordinal); err != nil {
					return err
				}
			case "exemption_expired": // Desired exemption expiry is driven by the master clock.
			default:
				return xnode.ErrInvalid
			}
			cursor = sequence
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO xnode_firewall_cursors(node_id,namespace,sequence) VALUES(?,?,?) ON CONFLICT(node_id,namespace) DO UPDATE SET sequence=excluded.sequence`, node, namespace, cursor); err != nil {
			return err
		}
		return queueFirewallFleetTx(ctx, tx, now)
	})
	if result == nil {
		for key, window := range s.firewallWindows {
			if !window.start.Add(time.Duration(window.rule.WindowSeconds) * time.Second).After(now) {
				delete(s.firewallWindows, key)
			}
		}
		for key, window := range staged {
			s.firewallWindows[key] = window
		}
	}
	return result
}
func authorizeDetectionTx(ctx context.Context, tx *sql.Tx, prefix, ip string, event wire.FirewallEvent, policy wire.FirewallPolicy, now time.Time) error {
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT active_bans FROM xnode_firewall_clock WHERE singleton=1`).Scan(&active); err != nil {
		return err
	}
	if active >= 1000000 {
		return xnode.ErrUnavailable
	}
	var existing string
	err := tx.QueryRowContext(ctx, `SELECT generation FROM xnode_firewall_bans WHERE prefix=? AND released_version=0`, prefix).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	generation, err := ids.New()
	if err != nil {
		return err
	}
	version, err := firewallVersion(ctx, tx)
	if err != nil {
		return err
	}
	if existing != "" {
		if _, err = tx.ExecContext(ctx, `UPDATE xnode_firewall_bans SET released_version=?,released_at=? WHERE generation=?`, version, stamp(now), existing); err != nil {
			return err
		}
	}
	audience, err := firewallAudience(ctx, tx)
	if err != nil {
		return err
	}
	empty, err := roaring.New().ToBytes()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_firewall_bans(generation,prefix,ip,rule,detected_count,detected_at,until_at,refresh_seconds,issued_version,participants,expired) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, generation, prefix, ip, event.Rule, fmt.Sprint(event.Count), stamp(event.Time), stamp(now.Add(policy.BanDuration())), int(policy.BanDuration()/time.Second), version, audience, empty)
	return err
}
