package database

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/netip"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Search streams active records before applying the result limit, so CIDR
// matches beyond earlier pages are visible without materializing address ranges.
func (s *Store) SearchFirewallBans(ctx context.Context, query netip.Prefix, cursor string, limit int) (xnode.FirewallBanPage, error) {
	page := xnode.FirewallBanPage{Items: []xnode.FirewallBanView{}}
	if limit < 1 || limit > 100 {
		return page, xnode.ErrInvalid
	}
	var after [2]string
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil || json.Unmarshal(raw, &after) != nil || after[0] == "" || after[1] == "" {
			return page, xnode.ErrInvalid
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT prefix,generation,ip,rule,detected_count,detected_at,until_at,refresh_seconds,participants,expired FROM xnode_firewall_bans WHERE released_version=0 AND (?='' OR detected_at<? OR (detected_at=? AND generation<?)) ORDER BY detected_at DESC,generation DESC`, cursor, after[0], after[0], after[1])
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var value xnode.FirewallBanView
		var detected, until string
		var participants, expired []byte
		if err = rows.Scan(&value.Prefix, &value.Generation, &value.IP, &value.Rule, &value.Count, &detected, &until, &value.RefreshSeconds, &participants, &expired); err != nil {
			return page, err
		}
		prefix, err := netip.ParsePrefix(value.Prefix)
		if err != nil {
			return page, err
		}
		if query.IsValid() && !prefix.Overlaps(query) {
			continue
		}
		if len(page.Items) == limit {
			raw, err := json.Marshal(after)
			if err != nil {
				return page, err
			}
			page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
			return page, nil
		}
		value.DetectedAt, err = time.Parse(time.RFC3339Nano, detected)
		if err != nil {
			return page, err
		}
		value.Until, err = time.Parse(time.RFC3339Nano, until)
		if err != nil {
			return page, err
		}
		p, err := firewallBitmap(participants)
		if err != nil {
			return page, err
		}
		e, err := firewallBitmap(expired)
		if err != nil {
			return page, err
		}
		p.AndNot(e)
		value.PendingNodes = int(p.GetCardinality())
		page.Items = append(page.Items, value)
		after = [2]string{detected, value.Generation}
	}
	return page, rows.Err()
}
