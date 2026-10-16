package xnode_test

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestSubscriptionTemplatesValidateAssignmentsAndRejectStaleWrites(t *testing.T) {
	f := controlFixture(t, false)
	ctx := context.Background()
	config, err := f.service.SubscriptionTemplates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	config.Assignments[f.user.ID] = map[string]string{"xray": config.Defaults["xray"]}
	for _, item := range config.Templates {
		raw, validation, err := f.service.PreviewSubscriptionTemplate(item)
		if err != nil || !validation.Valid || len(raw) == 0 {
			t.Fatalf("preview %s=%+v,%v", item.Format, validation, err)
		}
	}
	saved, err := f.service.SaveSubscriptionTemplates(ctx, f.user.ID, config)
	if err != nil || saved.Revision != 1 {
		t.Fatalf("save=%+v,%v", saved, err)
	}
	if _, err = f.service.SaveSubscriptionTemplates(ctx, f.user.ID, config); !errors.Is(err, xnode.ErrConflict) {
		t.Fatalf("stale overwrite accepted: %v", err)
	}
	saved.Templates = saved.Templates[1:]
	if _, err = f.service.SaveSubscriptionTemplates(ctx, f.user.ID, saved); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatalf("dangling default accepted: %v", err)
	}
}
