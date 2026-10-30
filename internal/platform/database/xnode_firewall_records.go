package database

import (
	"container/heap"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type firewallRecordCursor struct {
	Filter string    `json:"filter"`
	Time   time.Time `json:"time"`
	ID     string    `json:"id"`
}

// SearchFirewallRecords merges volatile observations and authoritative bans.
// It keeps only one bounded page in memory while scanning persistent bans.
func (s *Store) SearchFirewallRecords(ctx context.Context, request xnode.FirewallRecordSearch, now time.Time) (xnode.FirewallRecordPage, error) {
	page := xnode.FirewallRecordPage{Items: []xnode.FirewallRecord{}}
	if request.Limit < 1 || request.Limit > 100 || len(request.Cursor) > 512 {
		return page, xnode.ErrInvalid
	}
	predicate, err := xnode.CompileFirewallFilter(request.Filter)
	if err != nil {
		return page, err
	}
	raw, err := json.Marshal(request.Filter)
	if err != nil {
		return page, err
	}
	digest := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(digest[:])
	var after firewallRecordCursor
	if request.Cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(request.Cursor)
		if err != nil || json.Unmarshal(decoded, &after) != nil || after.Filter != fingerprint || after.Time.IsZero() || after.ID == "" {
			return page, xnode.ErrInvalid
		}
	}
	windows := s.firewallRecordWindows(now)
	selected := &firewallRecordHeap{}
	heap.Init(selected)
	consider := func(record xnode.FirewallRecord) {
		if !predicate(record) || !after.Time.IsZero() && !firewallRecordLess(record.DetectedAt, record.ID, after.Time, after.ID) {
			return
		}
		if selected.Len() < request.Limit+1 {
			heap.Push(selected, record)
		} else if firewallRecordLess((*selected)[0].DetectedAt, (*selected)[0].ID, record.DetectedAt, record.ID) {
			(*selected)[0] = record
			heap.Fix(selected, 0)
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT prefix,generation,ip,rule,detected_count,detected_at,until_at,participants,expired FROM xnode_firewall_bans WHERE released_version=0`)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		record, err := scanFirewallRecord(rows)
		if err != nil {
			return page, err
		}
		delete(windows, record.Prefix)
		consider(record)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	for _, records := range windows {
		for _, record := range records {
			consider(record)
		}
	}
	items := []xnode.FirewallRecord(*selected)
	sort.Slice(items, func(i, j int) bool {
		return firewallRecordLess(items[j].DetectedAt, items[j].ID, items[i].DetectedAt, items[i].ID)
	})
	if len(items) > request.Limit {
		items = items[:request.Limit]
		last := items[len(items)-1]
		raw, err = json.Marshal(firewallRecordCursor{Filter: fingerprint, Time: last.DetectedAt, ID: last.ID})
		if err != nil {
			return page, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	page.Items = items
	return page, nil
}

func (s *Store) firewallRecordWindows(now time.Time) map[string][]xnode.FirewallRecord {
	out := map[string][]xnode.FirewallRecord{}
	s.firewallDetectionMu.Lock()
	defer s.firewallDetectionMu.Unlock()
	for key, window := range s.firewallWindows {
		if !window.start.Add(time.Duration(window.rule.WindowSeconds) * time.Second).After(now) {
			continue
		}
		ip, rule, ok := strings.Cut(key, "|")
		if !ok {
			continue
		}
		prefix, err := wire.FirewallPrefix(ip)
		if err != nil {
			continue
		}
		address, err := netip.ParseAddr(ip)
		if err != nil {
			continue
		}
		until := window.start.Add(time.Duration(window.rule.WindowSeconds) * time.Second)
		record := xnode.FirewallRecord{ID: "r:" + key, IP: ip, Prefix: netip.PrefixFrom(address, address.BitLen()).String(), Rule: rule, Count: strconv.FormatUint(window.count, 10), DetectedAt: window.time, WindowUntil: &until}
		out[prefix.String()] = append(out[prefix.String()], record)
	}
	return out
}

func scanFirewallRecord(rows rowScanner) (xnode.FirewallRecord, error) {
	var record xnode.FirewallRecord
	var detected, until string
	var participants, expired []byte
	err := rows.Scan(&record.Prefix, &record.Generation, &record.IP, &record.Rule, &record.Count, &detected, &until, &participants, &expired)
	if err != nil {
		return record, err
	}
	if _, err = netip.ParsePrefix(record.Prefix); err != nil {
		return record, err
	}
	if record.DetectedAt, err = time.Parse(time.RFC3339Nano, detected); err != nil {
		return record, err
	}
	deadline, err := time.Parse(time.RFC3339Nano, until)
	if err != nil {
		return record, err
	}
	p, err := firewallBitmap(participants)
	if err != nil {
		return record, err
	}
	e, err := firewallBitmap(expired)
	if err != nil {
		return record, err
	}
	p.AndNot(e)
	pending := int(p.GetCardinality())
	record.ID, record.Banned, record.Until, record.PendingNodes = "b:"+record.Generation, true, &deadline, &pending
	return record, nil
}
