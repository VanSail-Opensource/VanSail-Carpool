package xnode_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func stringID(identity xnode.Identity) string { return strconv.FormatInt(identity.ProxyUserID, 10) }
func TestRegistrationDoesNotAuthorizeProxyAccess(t *testing.T) {
	f := controlFixture(t, false)
	config, err := f.service.Compile(context.Background(), f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Users) != 0 {
		t.Fatal("registration granted proxy access")
	}
}

func TestUtilityListenersCannotBeAssignedToMembersOrSubscriptions(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	profile, err := f.service.ResolveConfiguration(ctx, f.node.ConfigID)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	_ = json.Unmarshal(profile.Document, &native)
	native["inbounds"] = append(native["inbounds"].([]any), map[string]any{"tag": "utility", "protocol": "tunnel", "listen": "127.0.0.1", "port": 19090, "settings": map[string]any{"address": "127.0.0.1", "port": 8080, "network": "tcp"}})
	profile.Document, _ = json.Marshal(native)
	if _, err = f.service.SaveConfiguration(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveGroup(ctx, xnode.Group{Name: "Invalid", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: f.node.ID, InboundTag: "utility"}}}); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatalf("utility group accepted: %v", err)
	}
	if _, err = f.service.SaveEndpoint(ctx, xnode.Endpoint{Name: "Invalid", NodeID: f.node.ID, InboundTag: "utility", Address: "proxy.example", Port: 19090, Enabled: true}); !errors.Is(err, xnode.ErrInvalid) {
		t.Fatalf("utility subscription endpoint accepted: %v", err)
	}
}
func TestPaidAccountsOwnRoutingTagsAndMatchers(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	out := json.RawMessage(`[{"tag":"peer","protocol":"trojan","settings":{"servers":[{"address":"1.1.1.1","port":443,"password":"owned"}]},"streamSettings":{"network":"tcp","security":"tls","tlsSettings":{"serverName":"proxy.example"}}}]`)
	value, err := f.service.SaveOwnedOutbounds(ctx, f.user.ID, f.node.ID, 0, out)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.SaveOwnedRouting(ctx, f.user.ID, f.node.ID, value.Revision, json.RawMessage(`[{"outboundTag":"direct"}]`)); err == nil {
		t.Fatal("foreign outbound accepted")
	}
	if _, err = f.service.SaveOwnedRouting(ctx, f.user.ID, f.node.ID, value.Revision, json.RawMessage(`[{"outboundTag":"peer","user":["someone-else"]}]`)); err == nil {
		t.Fatal("foreign user matcher accepted")
	}
	value, err = f.service.SaveOwnedRouting(ctx, f.user.ID, f.node.ID, value.Revision, json.RawMessage(`[{"outboundTag":"peer","user":[""]}]`))
	if err != nil {
		t.Fatal(err)
	}
	config, err := f.service.Compile(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Users) != 1 || len(config.MemberOutbounds) != 1 || config.MemberOutbounds[0] != "peer-alice" {
		t.Fatalf("owner expansion=%+v", config.MemberOutbounds)
	}
	var root struct {
		Routing struct {
			Rules []struct {
				User        []string `json:"user"`
				OutboundTag string   `json:"outboundTag"`
			} `json:"rules"`
		} `json:"routing"`
	}
	if err = json.Unmarshal(config.Xray, &root); err != nil {
		t.Fatal(err)
	}
	rule := root.Routing.Rules[0]
	matcher, err := regexp.Compile(rule.User[0][len("regexp:"):])
	if err != nil {
		t.Fatal(err)
	}
	own := "user@" + stringID(f.identity) + ":0123456789abcdef0123456789abcdef@0123456789abcdef01234567"
	if !matcher.MatchString(own) || matcher.MatchString("prefix"+own) || matcher.MatchString(own+"suffix") || matcher.MatchString("user@999:"+own) {
		t.Fatal("matcher did not isolate the authenticated numeric owner")
	}
	if _, err = f.service.SaveOwnedOutbounds(ctx, f.user.ID, f.node.ID, value.Revision, json.RawMessage(`[]`)); !errors.Is(err, xnode.ErrConflict) {
		t.Fatal("dangling route accepted")
	}
	if _, err = f.service.SaveOwnedOutbounds(ctx, f.user.ID, f.node.ID, value.Revision, json.RawMessage(`[{"tag":"private","protocol":"socks","settings":{"servers":[{"address":"127.0.0.1","port":1080}]}}]`)); err == nil {
		t.Fatal("private proxy accepted")
	}
	if _, err = f.service.SaveOwnedOutbounds(ctx, f.user.ID, f.node.ID, value.Revision, json.RawMessage(`[{"tag":"cycle","protocol":"socks","settings":{"servers":[{"address":"1.1.1.1","port":1080}]},"proxySettings":{"tag":"cycle"}}]`)); err == nil {
		t.Fatal("proxy cycle accepted")
	}
}
func TestSubscriptionRevocationPreservesProxyCredentials(t *testing.T) {
	f := controlFixture(t, true)
	ctx := context.Background()
	before, err := f.service.Compile(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	old, err := f.service.OwnedSubscription(ctx, f.user.ID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := f.service.RevokeSubscription(ctx, stringID(f.identity))
	if err != nil || next == old.URL {
		t.Fatalf("token did not rotate: %v", err)
	}
	oldToken := old.URL[strings.LastIndex(old.URL, "/")+1:]
	if _, err = f.service.BearerMetadata(ctx, oldToken); !errors.Is(err, xnode.ErrNotFound) {
		t.Fatal("revoked bearer token accepted")
	}
	newToken := next[strings.LastIndex(next, "/")+1:]
	if _, err = f.service.BearerMetadata(ctx, newToken); err != nil {
		t.Fatal(err)
	}
	after, err := f.service.Compile(ctx, f.node.ID)
	if err != nil {
		t.Fatal(err)
	}
	if before.Users[0].UUID != after.Users[0].UUID {
		t.Fatal("subscription revocation rotated proxy UUID")
	}
}
