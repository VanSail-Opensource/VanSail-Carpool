package app

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
)

func TestMemberInventorySharedExcludedHiddenAndCountries(t *testing.T) {
	raws := []json.RawMessage{
		json.RawMessage(`{"finalRemark":"Shared","address":"proxy.example","port":443,"protocol":"vless","metadata":{"uuid":"shared","configProfileInboundUuid":"inbound","isHidden":true}}`),
		json.RawMessage(`{"finalRemark":"Excluded","address":"proxy.example","port":443,"protocol":"vless","metadata":{"uuid":"excluded","configProfileInboundUuid":"inbound"}}`),
	}
	hosts := []remnawave.Host{{UUID: "shared", Nodes: []string{"de", "nl"}}, {UUID: "excluded", ExcludedInternalSquads: []string{"a"}}}
	squads := []remnawave.InternalSquad{{UUID: "a", Name: "A", Inbounds: []remnawave.NodeInbound{{UUID: "inbound"}}}, {UUID: "b", Name: "B", Inbounds: []remnawave.NodeInbound{{UUID: "inbound"}}}, {UUID: "disabled", Name: "Disabled", Inbounds: []remnawave.NodeInbound{{UUID: "inbound"}}}}
	active := []remnawave.SquadSummary{{UUID: "a"}, {UUID: "b"}, {UUID: "disabled"}}
	keys := &remnawave.ConnectionKeys{HiddenKeys: []string{"vless://owner@proxy.example:443#Shared"}, EnabledKeys: []string{"vless://owner@proxy.example:443#Excluded"}, DisabledKeys: []string{"vless://other@proxy.example:443#Shared"}}
	result, err := projectMemberInventory(raws, active, []string{"a", "b"}, hosts, squads, []remnawave.Node{{UUID: "de", CountryCode: "DE"}, {UUID: "nl", CountryCode: "NL"}}, keys)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hosts) != 2 || len(result.Squads) != 2 {
		t.Fatal("inventory dropped or duplicated hosts/squads")
	}
	for _, host := range result.Hosts {
		if host.UUID == "shared" {
			if !reflect.DeepEqual(host.CountryCodes, []string{"DE", "NL"}) || !reflect.DeepEqual(host.SquadUUIDs, []string{"a", "b"}) || host.Link == nil || *host.Link != keys.HiddenKeys[0] {
				t.Fatal("shared hidden host projection incorrect")
			}
		} else if !reflect.DeepEqual(host.SquadUUIDs, []string{"b"}) {
			t.Fatal("excluded squad acquired host")
		}
	}
	if _, err := projectMemberInventory(append(raws, raws[0]), active, []string{"a"}, hosts, squads, nil, keys); err == nil {
		t.Fatal("duplicate host identity accepted")
	}
}

func TestMemberNativeLinksRejectAmbiguityAndDisabledKeys(t *testing.T) {
	host := memberResolvedHost{Protocol: "vless", Address: "::1", Port: 443, FinalRemark: "Tokyo edge"}
	link := "vless://owner@[::1]:443?type=tcp#Tokyo%20edge"
	for _, test := range []struct {
		name string
		keys *remnawave.ConnectionKeys
		want string
	}{
		{"hidden", &remnawave.ConnectionKeys{HiddenKeys: []string{link}}, ""},
		{"disabled", &remnawave.ConnectionKeys{DisabledKeys: []string{link}}, "CONNECTIVITY_LINK_UNAVAILABLE"},
		{"ambiguous", &remnawave.ConnectionKeys{EnabledKeys: []string{link, "vless://owner@[::1]:443?type=ws#Tokyo%20edge"}}, "CONNECTIVITY_LINK_AMBIGUOUS"},
		{"absent", nil, "CONNECTIVITY_LINK_UNAVAILABLE"},
		{"wrong scheme", &remnawave.ConnectionKeys{EnabledKeys: []string{"trojan://owner@[::1]:443#Tokyo%20edge"}}, "CONNECTIVITY_LINK_UNAVAILABLE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			value, code := memberHostLink(host, test.keys)
			if code != test.want || (test.want != "" && value != nil) {
				t.Fatalf("link=%v code=%s", value, code)
			}
		})
	}
}

func TestMemberCredentialRotationInvalidatesRead(t *testing.T) {
	before := &remnawave.User{ID: 47, ShortUUID: "old", SubscriptionURL: "https://subscription.example/old", Status: remnawave.UserStatusActive, ExpireAt: time.Now().Add(time.Hour), ActiveInternalSquads: []remnawave.SquadSummary{{UUID: "a"}}}
	after := *before
	if !sameSubscriptionCredentials(before, &after) {
		t.Fatal("stable read rejected")
	}
	after.ShortUUID = "new"
	if sameSubscriptionCredentials(before, &after) {
		t.Fatal("rotated credentials retained")
	}
	after = *before
	at := time.Now()
	after.SubRevokedAt = &at
	if sameSubscriptionCredentials(before, &after) {
		t.Fatal("revocation timestamp ignored")
	}
	after = *before
	after.ActiveInternalSquads = []remnawave.SquadSummary{{UUID: "b"}}
	if sameSubscriptionCredentials(before, &after) {
		t.Fatal("access change ignored")
	}
}
