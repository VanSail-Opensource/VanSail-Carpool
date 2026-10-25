package database

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// newTestStore uses SQLite and the hosted PostgreSQL contract backend.
func TestCatalogTopologyEpochTracksMetadataWithoutLiveState(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if err := s.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "config", Name: "Config", DocumentCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveXNodeNode(ctx, xnode.Node{ID: "node", Name: "Node", ConfigID: "config", MultiplierFP: 1_000_000}); err != nil {
		t.Fatal(err)
	}
	version := func() int64 {
		value, _, err := s.XNodeCompileVersion(ctx, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, statement := range []string{
		`UPDATE xnode_nodes SET name='Renamed' WHERE id='node'`,
		`UPDATE xnode_nodes SET country_code='JP' WHERE id='node'`,
		`UPDATE xnode_nodes SET provider_name='Provider' WHERE id='node'`,
		`UPDATE xnode_nodes SET multiplier_fp=2 WHERE id='node'`,
	} {
		before := version()
		if _, err := s.DB().ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
		if version() != before+1 {
			t.Fatal("metadata did not advance revision", statement)
		}
	}
	before := version()
	if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_nodes SET name=name,desired_revision=desired_revision+1,updated_at=? WHERE id='node'`, stamp(time.Now())); err != nil {
		t.Fatal(err)
	}
	if version() != before {
		t.Fatal("live bookkeeping invalidated catalog/compile cache")
	}
}
