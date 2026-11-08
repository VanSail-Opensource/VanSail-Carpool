package database

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/RoaringBitmap/roaring/v2"
)

const firewallSweepLegacyQuery = `SELECT key,kind,target,generation,participants,expired FROM (
 SELECT 'b:'||generation AS key,'xnode_firewall_bans' AS kind,prefix AS target,generation,participants,expired FROM xnode_firewall_bans WHERE released_version=0
 UNION ALL SELECT 's:'||id,'xnode_firewall_sources',id,generation,participants,expired FROM xnode_firewall_sources WHERE released_version=0
 ) page WHERE key>$1 ORDER BY key LIMIT 256`

func firewallSeekFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.ReconcileFirewallMember(ctx, "retired", true, true, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.ReconcileFirewallMember(ctx, "retired", false, false, 0); err != nil {
		t.Fatal(err)
	}
	var ordinal uint32
	if err := s.db.QueryRowContext(ctx, `SELECT ordinal FROM xnode_firewall_members WHERE node_id='retired'`).Scan(&ordinal); err != nil {
		t.Fatal(err)
	}
	participants, err := roaring.BitmapOf(ordinal).ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	at := stamp(time.Now().UTC())
	// Empty identifiers exercise the inclusive index bound; released generations
	// between active rows must not consume the per-bank page limit.
	for _, query := range []string{
		`INSERT INTO xnode_firewall_bans(generation,prefix,ip,rule,detected_count,detected_at,until_at,refresh_seconds,issued_version,released_version,participants,expired)
 SELECT CASE WHEN i=1 THEN '' ELSE lpad(i::text,4,'0') END,'198.18.'||(i/256)::text||'.'||(i%256)::text||'/32','198.18.0.1','closed_port','1',$1,$1,60,1,CASE WHEN i%11=0 THEN 1 ELSE 0 END,$2,$3 FROM generate_series(1,520) i`,
		`INSERT INTO xnode_firewall_sources(id,generation,until_at,refresh_seconds,issued_version,released_version,participants,expired)
 SELECT CASE WHEN i=1 THEN '' ELSE lpad(i::text,4,'0') END,'generation-'||i::text,$1,60,1,CASE WHEN i%11=0 THEN 1 ELSE 0 END,$2,$3 FROM generate_series(1,300) i`,
	} {
		if _, err := s.db.ExecContext(ctx, query, at, participants, []byte{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_firewall_clock SET version=1,source_version=1 WHERE singleton=1`); err != nil {
		t.Fatal(err)
	}
	return s
}

func firewallSeekKeys(t *testing.T, s *Store, query string, args ...any) []string {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), `SELECT key FROM (`+query+`) selected ORDER BY key`, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return keys
}

func TestFirewallSweepSeekPreservesBankOrderAndBoundaries(t *testing.T) {
	s := firewallSeekFixture(t)
	for _, cursor := range []string{"", "b:", "b:0256", "b:0511", "b:zzzz", "s:", "s:0256", "s:zzzz", "z:"} {
		t.Run(cursor, func(t *testing.T) {
			banAfter, bans := firewallSweepRange("b:", cursor)
			sourceAfter, sources := firewallSweepRange("s:", cursor)
			got := firewallSeekKeys(t, s, firewallSweepPageQuery, cursor, boolInt(bans), banAfter, boolInt(sources), sourceAfter)
			want := firewallSeekKeys(t, s, firewallSweepLegacyQuery, cursor)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("cursor %q: got %v; want %v", cursor, got, want)
			}
		})
	}
}

func TestFirewallSweepSeekCompletesFullExpiryCycle(t *testing.T) {
	s := firewallSeekFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for page := 0; page < 5; page++ {
		keys := firewallSeekKeys(t, s, firewallSweepLegacyQuery, s.firewallSweepCursor)
		wantCursor := ""
		if len(keys) == 256 {
			wantCursor = keys[len(keys)-1]
		}
		var before, after int
		count := `SELECT (SELECT COUNT(*) FROM xnode_firewall_bans WHERE released_version=0)+(SELECT COUNT(*) FROM xnode_firewall_sources WHERE released_version=0)`
		if err := s.db.QueryRowContext(ctx, count).Scan(&before); err != nil {
			t.Fatal(err)
		}
		if err := s.SweepFirewall(ctx, now); err != nil {
			t.Fatal(err)
		}
		if err := s.db.QueryRowContext(ctx, count).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before-after != len(keys) || s.firewallSweepCursor != wantCursor {
			t.Fatalf("page %d: expired %d/%d, cursor %q/%q", page, before-after, len(keys), s.firewallSweepCursor, wantCursor)
		}
		if wantCursor == "" {
			if after != 0 {
				t.Fatal("full cycle skipped active generations", after)
			}
			return
		}
	}
	t.Fatal("sweep did not wrap")
}

func TestFirewallSweepSeekRollsBackPageAndRetriesSameCursor(t *testing.T) {
	s := firewallSeekFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	if err := s.SweepFirewall(ctx, now); err != nil {
		t.Fatal(err)
	}
	cursor, retention := s.firewallSweepCursor, s.firewallRetentionAt
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_firewall_sources SET participants=$1 WHERE id=''`, []byte{1}); err != nil {
		t.Fatal(err)
	}
	if err := s.SweepFirewall(ctx, now.Add(time.Second)); err == nil {
		t.Fatal("corrupt source bitmap committed the page")
	}
	var released int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_firewall_bans WHERE released_version>1`).Scan(&released); err != nil {
		t.Fatal(err)
	}
	if released != 256 || s.firewallSweepCursor != cursor || !s.firewallRetentionAt.Equal(retention) {
		t.Fatal("failed page advanced expiry or cursors", released, s.firewallSweepCursor)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE xnode_firewall_sources SET participants=$1 WHERE id=''`, []byte{}); err != nil {
		t.Fatal(err)
	}
	if err := s.SweepFirewall(ctx, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.firewallSweepCursor == cursor || s.firewallSweepCursor == "" {
		t.Fatal("recovery failed to resume the same page", s.firewallSweepCursor)
	}
}
