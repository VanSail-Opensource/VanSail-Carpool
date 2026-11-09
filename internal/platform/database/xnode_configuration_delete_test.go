package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestConfigurationDeleteProtectsActiveAndHistoricalNodeReferences(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	config := xnode.Configuration{ID: "referenced", Name: "Referenced configuration", DocumentCipher: "retained-encrypted-native-document"}
	if err := s.SaveXNodeConfiguration(ctx, config); err != nil {
		t.Fatal(err)
	}
	node := xnode.Node{ID: "node", Name: "Node", ConfigID: config.ID, SettingsCipher: "retained-encrypted-settings", Enabled: true}
	if err := s.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"active", "historical"} {
		t.Run(state, func(t *testing.T) {
			if state == "historical" {
				if _, err := s.DB().ExecContext(ctx, `UPDATE xnode_nodes SET enabled=0,retired_at=$1 WHERE id=$2`, stamp(time.Now().UTC()), node.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.DeleteXNodeConfiguration(ctx, config.ID); !errors.Is(err, xnode.ErrConflict) {
				t.Fatal("referenced configuration did not report conflict", err)
			}
			retained, err := s.XNodeConfiguration(ctx, config.ID)
			if err != nil || retained.DocumentCipher != config.DocumentCipher {
				t.Fatal("failed deletion changed the native configuration", err)
			}
			retainedNode, err := s.XNodeNode(ctx, node.ID)
			if err != nil || retainedNode.ConfigID != config.ID || retainedNode.SettingsCipher != node.SettingsCipher {
				t.Fatal("failed deletion changed the node reference", err)
			}
		})
	}
}

func TestConfigurationDeleteRemovesOnlyUnusedDocumentAndSnippetBindings(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	snippet := xnode.Snippet{ID: "shared", Name: "Shared outbound", Kind: "outbound", DocumentCipher: "retained-encrypted-snippet"}
	if err := s.SaveXNodeSnippet(ctx, snippet); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"unused", "retained"} {
		if err := s.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: id, Name: id, DocumentCipher: "encrypted-" + id, SnippetIDs: []string{snippet.ID}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteXNodeConfiguration(ctx, "unused"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.XNodeConfiguration(ctx, "unused"); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal("unused configuration survived deletion", err)
	}
	var removedBindings int
	if err := s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_config_snippets WHERE config_id='unused'`).Scan(&removedBindings); err != nil || removedBindings != 0 {
		t.Fatal("deleted configuration retained snippet bindings", err)
	}
	retained, err := s.XNodeConfiguration(ctx, "retained")
	if err != nil || len(retained.SnippetIDs) != 1 || retained.SnippetIDs[0] != snippet.ID || retained.DocumentCipher != "encrypted-retained" {
		t.Fatal("unrelated configuration or binding changed", err)
	}
	retainedSnippet, err := s.XNodeSnippet(ctx, snippet.ID)
	if err != nil || retainedSnippet.DocumentCipher != snippet.DocumentCipher {
		t.Fatal("shared snippet was deleted or rewritten", err)
	}
	if err := s.DeleteXNodeConfiguration(ctx, "unused"); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal("missing configuration did not report not found", err)
	}
}
