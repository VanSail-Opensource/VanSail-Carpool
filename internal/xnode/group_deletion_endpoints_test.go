package xnode

import (
	"encoding/json"
	"testing"
)

func TestDeletedEndpointGroupPreservesDenyAllPolicy(t *testing.T) {
	raw, changed, err := cleanEndpointGroup(json.RawMessage(`{"allowedGroups":["retired"],"excludedGroups":["retired","keep"],"legacyHost":{"internalSquads":{"mode":"ALLOW_ONLY","squads":["retired"]}}}`), "retired")
	if err != nil || !changed {
		t.Fatal(string(raw), changed, err)
	}
	groups, err := filterEndpointGroups([]string{"new-group", "keep"}, raw)
	if err != nil || len(groups) != 0 {
		t.Fatal("empty allow list widened access", groups, err)
	}
	if _, changed, err = cleanEndpointGroup(raw, "retired"); err != nil || changed {
		t.Fatal("cleanup is not idempotent", changed, err)
	}
}
