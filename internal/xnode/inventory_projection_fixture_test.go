package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
)

type inventoryTestSources interface {
	Repository
	paygRepository
	hourlyPrincipalRepository
}

type inventoryProjectionRepository struct {
	inventoryTestSources
	node                                      Node
	configuration                             Configuration
	snippet                                   Snippet
	groups                                    []Group
	endpoints                                 []Endpoint
	identities                                map[int64]Identity
	accounts                                  []PAYGAccount
	hourly                                    []HourlyPrincipal
	purchase                                  *model.Purchase
	period                                    Period
	disabled                                  []string
	restricted                                bool
	configurationErr                          error
	nodeReads                                 map[string]int
	groupReads, endpointReads, configReads    int
	snippetReads, identityScans, accountReads int
}

func (r *inventoryProjectionRepository) UserByID(context.Context, string) (model.User, error) {
	return model.User{ID: "member", OnboardingState: "complete"}, nil
}
func (r *inventoryProjectionRepository) XNodeIdentityByUser(context.Context, string) (Identity, error) {
	return r.identities[1], nil
}
func (r *inventoryProjectionRepository) XNodeIdentity(_ context.Context, id int64) (Identity, error) {
	return r.identities[id], nil
}
func (r *inventoryProjectionRepository) XNodeIdentities(context.Context) ([]Identity, error) {
	r.identityScans++
	return nil, ErrUnavailable
}
func (r *inventoryProjectionRepository) XNodeNode(_ context.Context, id string) (Node, error) {
	r.nodeReads[id]++
	if id != r.node.ID {
		return Node{}, ErrNotFound
	}
	return r.node, nil
}
func (r *inventoryProjectionRepository) XNodeGroups(context.Context) ([]Group, error) {
	r.groupReads++
	return r.groups, nil
}
func (r *inventoryProjectionRepository) XNodeEndpoints(context.Context) ([]Endpoint, error) {
	r.endpointReads++
	return slices.Clone(r.endpoints), nil
}
func (r *inventoryProjectionRepository) XNodeConfiguration(context.Context, string) (Configuration, error) {
	r.configReads++
	return r.configuration, r.configurationErr
}
func (r *inventoryProjectionRepository) XNodeSnippet(context.Context, string) (Snippet, error) {
	r.snippetReads++
	return r.snippet, nil
}
func (r *inventoryProjectionRepository) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return r.purchase, nil
}
func (r *inventoryProjectionRepository) ListPurchases(context.Context, string) ([]model.Purchase, error) {
	if r.purchase != nil {
		return []model.Purchase{*r.purchase}, nil
	}
	return nil, nil
}
func (r *inventoryProjectionRepository) XNodePeriod(context.Context, string) (Period, error) {
	return r.period, nil
}
func (r *inventoryProjectionRepository) UserDisabledSquads(context.Context, string) ([]string, error) {
	return r.disabled, nil
}
func (r *inventoryProjectionRepository) XNodeUserRestricted(context.Context, string) (bool, error) {
	return r.restricted, nil
}
func (r *inventoryProjectionRepository) XNodePAYGAccounts(context.Context, string, string) ([]PAYGAccount, error) {
	r.accountReads++
	return r.accounts, nil
}
func (r *inventoryProjectionRepository) ListSquadProductOverrides(context.Context, bool) ([]model.SquadProduct, error) {
	return []model.SquadProduct{{GroupID: "payg-a", BillingMode: "payg"}, {GroupID: "payg-b", BillingMode: "payg"}}, nil
}
func (r *inventoryProjectionRepository) XNodeHourlyPrincipals(context.Context, string, string) ([]HourlyPrincipal, error) {
	return r.hourly, nil
}

func inventoryProjectionFixture(t *testing.T) (*Service, *inventoryProjectionRepository) {
	t.Helper()
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	seal := func(scope, raw string) string {
		t.Helper()
		cipher, err := vault.Encrypt(scope, raw)
		if err != nil {
			t.Fatal(err)
		}
		return cipher
	}
	r := &inventoryProjectionRepository{nodeReads: map[string]int{}, identities: map[int64]Identity{1: {ProxyUserID: 1, UserID: "member"}}}
	r.node = Node{ID: "node", ConfigID: "configuration", Enabled: true, SettingsCipher: seal("xnode.node.node", `{}`)}
	r.configuration = Configuration{ID: "configuration", SnippetIDs: []string{"snippet"}, DocumentCipher: seal("xnode.config.configuration", `{"inbounds":[{"tag":"paid","protocol":"vless","settings":{"decryption":"none"},"streamSettings":{"network":"ws","wsSettings":{"path":"/native"}}}],"outbounds":[]}`)}
	r.snippet = Snippet{ID: "snippet", Kind: "outbound", DocumentCipher: seal("xnode.snippet.snippet", `[{"tag":"direct","protocol":"freedom"}]`)}
	for index, group := range []string{"payg-a", "payg-b", "hourly"} {
		r.groups = append(r.groups, Group{ID: group, Enabled: true, Bindings: []GroupBinding{{NodeID: "node", InboundTag: "paid"}}})
		id := int64(index + 2)
		identity := Identity{ProxyUserID: id, ServiceName: "principal-" + group, PAYGOwnerID: "member"}
		if group == "hourly" {
			identity.PAYGOwnerID, identity.HourlyOwnerID = "", "member"
		}
		identity.CredentialsCipher = seal("xnode.identity."+identityScope(identity), fmt.Sprintf(`{"uuid":"00000000-0000-4000-8000-%012d"}`, id))
		r.identities[id] = identity
		if group != "hourly" {
			r.accounts = append(r.accounts, PAYGAccount{ProxyUserID: id, UserID: "member", NodeID: "node", GroupID: group})
		} else {
			r.hourly = []HourlyPrincipal{{EnrollmentID: "hourly-enrollment", OwnerID: "member", GroupID: group, ProxyUserID: id, Status: "active", PaidThrough: time.Now().Add(time.Hour)}}
		}
	}
	for i := range 8 {
		id := fmt.Sprintf("endpoint-%d", i)
		r.endpoints = append(r.endpoints, Endpoint{ID: id, NodeID: "node", InboundTag: "paid", Name: id, Address: "proxy.example", Port: 443, Enabled: true, OverridesCipher: seal("xnode.endpoint."+id, `{}`)})
	}
	// Foreign endpoints must be rejected by saved group bindings before any
	// native configuration or credential projection for that node is attempted.
	r.groups = append(r.groups, Group{ID: "foreign", Enabled: true, Bindings: []GroupBinding{{NodeID: "foreign", InboundTag: "other"}}})
	r.endpoints = append(r.endpoints, Endpoint{ID: "foreign", NodeID: "foreign", InboundTag: "other", Enabled: true, OverridesCipher: seal("xnode.endpoint.foreign", `{}`)})
	return NewService(r, cacheTestSettings{}, vault, nil, nil), r
}

func setInventoryOverrides(t *testing.T, s *Service, r *inventoryProjectionRepository, raw json.RawMessage) {
	t.Helper()
	for i := range r.endpoints {
		cipher, err := s.vault.Encrypt("xnode.endpoint."+r.endpoints[i].ID, string(raw))
		if err != nil {
			t.Fatal(err)
		}
		r.endpoints[i].OverridesCipher = cipher
	}
}
