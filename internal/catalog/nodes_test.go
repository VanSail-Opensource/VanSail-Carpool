package catalog

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func TestQuoteProjectsAccessibleNodes(t *testing.T) {
	t.Parallel()

	repository := &catalogQuoteRepository{
		catalogRepository: nodeCatalogRepository(),
		quote:             model.PurchaseQuote{ComboID: "core", ComboName: "Core"},
	}
	remote := &catalogNodeRemote{
		catalogProxy: &catalogProxy{},
		squads: []RemoteSquad{
			{UUID: "core-squad", Name: "Core"},
			{UUID: "shared-squad", Name: "Shared"},
			{UUID: "addon-squad", Name: "Extra"},
		},
		accessible: map[string][]string{
			"addon-squad":  {"berlin"},
			"core-squad":   {"zulu", "austin", "disabled"},
			"shared-squad": {"austin"},
		},
		nodes: []RemoteNode{
			{UUID: "zulu", Name: "Zulu", CountryCode: "US", ConsumptionMultiplier: 1.5},
			{UUID: "austin", Name: "Austin", CountryCode: "US", ActiveInboundUUIDs: []string{"inbound-1"}},
			{UUID: "berlin", Name: "Berlin", CountryCode: "DE", ConsumptionMultiplier: 0.8},
			{UUID: "disabled", Name: "Disabled", CountryCode: "DE", Disabled: true},
			{UUID: "unselected", Name: "Unselected", CountryCode: "AU"},
		},
	}
	service := NewService(repository, remote, time.Minute)
	quotedAt := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return quotedAt }

	quote, err := service.Quote(context.Background(), model.User{ID: "user-1", OnboardingState: "complete"}, "core", []string{"addon-squad"}, "grant-1")
	if err != nil {
		t.Fatalf("Quote(): %v", err)
	}
	wantNodes := []model.ProxyNode{
		{UUID: "berlin", Name: "Berlin", CountryCode: "DE", ConsumptionMultiplier: 0.8, Accessible: true},
		{UUID: "austin", Name: "Austin", CountryCode: "US", ActiveInboundUUIDs: []string{"inbound-1"}, Accessible: true},
		{UUID: "zulu", Name: "Zulu", CountryCode: "US", ConsumptionMultiplier: 1.5, Accessible: true},
	}
	if !reflect.DeepEqual(quote.AccessibleNodes, wantNodes) {
		t.Fatalf("Quote().AccessibleNodes = %#v, want %#v", quote.AccessibleNodes, wantNodes)
	}
	if !reflect.DeepEqual(remote.requestedSquads, []string{"addon-squad", "core-squad", "shared-squad"}) {
		t.Fatalf("accessible node requests = %v", remote.requestedSquads)
	}
	if !reflect.DeepEqual(repository.quoteInput, database.PurchaseInput{
		UserID: "user-1", ComboID: "core", AddonSquadIDs: []string{"addon-squad"}, CouponGrantID: "grant-1",
	}) {
		t.Fatalf("QuotePurchase input = %#v", repository.quoteInput)
	}
	if !repository.quoteAt.Equal(quotedAt) {
		t.Fatalf("QuotePurchase time = %s, want %s", repository.quoteAt, quotedAt)
	}
}

func TestQuoteAccessibleNodesHandlesUnavailableProviders(t *testing.T) {
	t.Parallel()

	testError := errors.New("upstream failure")
	tests := []struct {
		name      string
		configure func(*catalogNodeRemote)
		wantErr   error
	}{
		{
			name: "squad lookup failure",
			configure: func(remote *catalogNodeRemote) {
				remote.lookupErr = testError
			},
			wantErr: testError,
		},
		{
			name: "node list failure",
			configure: func(remote *catalogNodeRemote) {
				remote.nodesErr = testError
			},
			wantErr: testError,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := nodeCatalogRepository()
			remote := newCatalogNodeRemote()
			test.configure(remote)
			_, err := NewService(repository, remote, time.Minute).Catalog(context.Background())
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Catalog() node hydration error = %v, want %v", err, test.wantErr)
			}
		})
	}

	repository := &catalogQuoteRepository{catalogRepository: nodeCatalogRepository(), quote: model.PurchaseQuote{ComboID: "core"}}
	service := newCatalogServiceForTest(repository, &catalogProxy{})
	quote, err := service.Quote(context.Background(), model.User{ID: "user", OnboardingState: "complete"}, "core", []string{"addon-squad"}, "")
	if !errors.Is(err, ErrNoAccessibleNodes) || len(quote.AccessibleNodes) != 0 {
		t.Fatalf("Quote() without node provider = (%v, %v), want ErrNoAccessibleNodes", quote.AccessibleNodes, err)
	}
}

type catalogQuoteRepository struct {
	*catalogRepository
	quote      model.PurchaseQuote
	quoteErr   error
	quoteInput database.PurchaseInput
	quoteAt    time.Time
}

func (r *catalogQuoteRepository) QuotePurchase(_ context.Context, input database.PurchaseInput, at time.Time) (model.PurchaseQuote, error) {
	r.quoteInput, r.quoteAt = input, at
	return r.quote, r.quoteErr
}

type catalogNodeRemote struct {
	*catalogProxy
	squads          []RemoteSquad
	nodes           []RemoteNode
	accessible      map[string][]string
	lookupErr       error
	nodesErr        error
	requestedSquads []string
}

func (r *catalogNodeRemote) ListCatalogSquads(context.Context) ([]RemoteSquad, error) {
	return r.squads, nil
}

func (r *catalogNodeRemote) ListCatalogNodes(context.Context) ([]RemoteNode, error) {
	return r.nodes, r.nodesErr
}

func (r *catalogNodeRemote) AccessibleCatalogNodeUUIDs(_ context.Context, squadUUID string) ([]string, error) {
	r.requestedSquads = append(r.requestedSquads, squadUUID)
	if r.lookupErr != nil {
		return nil, r.lookupErr
	}
	return r.accessible[squadUUID], nil
}

func nodeCatalogRepository() *catalogRepository {
	return &catalogRepository{
		combos: []model.Combo{{
			ID: "core",
			IncludedSquads: []model.SquadProduct{
				{GroupID: "core-squad"},
				{GroupID: "shared-squad"},
			},
		}},
		addons: []model.SquadProduct{{ID: "addon-product", GroupID: "addon-squad", Visible: true}},
	}
}

func newCatalogNodeRemote() *catalogNodeRemote {
	return &catalogNodeRemote{
		catalogProxy: &catalogProxy{},
		squads: []RemoteSquad{
			{UUID: "core-squad", Name: "Core"},
			{UUID: "shared-squad", Name: "Shared"},
			{UUID: "addon-squad", Name: "Extra"},
		},
		accessible: map[string][]string{
			"core-squad": {"node-1"},
		},
	}
}
