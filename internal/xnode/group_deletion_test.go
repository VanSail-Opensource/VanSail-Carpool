package xnode_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNamedServicePreventsGroupDeletion(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	access := xnode.ServiceAccess{LimitBytes: 1000, Strategy: "NO_RESET", StartedAt: time.Now().UTC(), ExpiresAt: time.Now().Add(time.Hour), GroupIDs: []string{f.group.ID}, Disabled: true}
	if _, err := f.service.SaveNamedService(ctx, "", "retained-service", access); err != nil {
		t.Fatal(err)
	}
	groups, err := f.service.AdminGroups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range groups {
		if g.ID == f.group.ID && g.CanDelete {
			t.Fatal("disabled unexpired service permits deletion")
		}
	}
	if err = f.service.DeleteGroup(ctx, f.group.ID); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal(err)
	}
}
