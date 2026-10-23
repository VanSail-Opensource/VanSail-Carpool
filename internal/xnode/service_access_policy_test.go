package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestStandaloneAuthorizationKeepsQuotaAndGroupPolicies(t *testing.T) {
	for _, tc := range []struct {
		name        string
		limit, used int64
		blocked     bool
	}{
		{"finite", 1000, 100, false},
		{"unlimited", 0, 10000, false},
		{"exhausted", 1000, 1000, true},
		{"disabled identity", 1000, 100, true},
		{"disabled group", 1000, 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, identity, _ := standaloneFixture(t, tc.limit, tc.used, "NO_RESET")
			ctx := context.Background()
			if tc.name == "disabled identity" {
				if err := f.store.SetXNodeAccessDisabled(ctx, identity.ProxyUserID, true); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "disabled group" {
				f.group.Enabled = false
				if _, err := f.service.SaveGroup(ctx, f.group); err != nil {
					t.Fatal(err)
				}
			}
			compiled, err := f.service.Compile(ctx, f.node.ID)
			if err != nil {
				t.Fatal(err)
			}
			if tc.blocked {
				if len(compiled.Users) != 0 {
					t.Fatal("restricted standalone account remains authorized")
				}
				return
			}
			if len(compiled.Users) != 1 || compiled.Users[0].AccountingPeriod != identity.CurrentPeriodID || compiled.Users[0].ConnectionLimit != 0 || compiled.Users[0].SpeedLimitMbps != 0 {
				t.Fatal("standalone account inherited probe limits or lost accounting")
			}
		})
	}
}

func TestStandaloneSubscriptionHonorsHostACLAndExcludesProbeBearers(t *testing.T) {
	f, identity, _ := standaloneFixture(t, 1000, 100, "NO_RESET")
	ctx := context.Background()
	endpoint, err := f.service.SaveEndpoint(ctx, xnode.Endpoint{NodeID: f.node.ID, InboundTag: "paid", Name: "Restricted host", Address: "proxy.example", Port: 443, Enabled: true, Overrides: json.RawMessage(`{"legacyHost":{"internalSquads":{"mode":"ALLOW_ONLY","squads":["foreign"]}}}`)})
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := f.service.ServiceSubscription(ctx, stringID(identity))
	if err != nil || metadata.Active {
		t.Fatalf("standalone subscription bypassed host ACL: %v", err)
	}
	probe, err := f.service.AuthorizeService(ctx, "bounded-probe", endpoint, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	link, err := f.service.RevokeSubscription(ctx, stringID(probe))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.BearerMetadata(ctx, link[strings.LastIndex(link, "/")+1:]); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatalf("probe acquired standalone subscription access: %v", err)
	}
}
