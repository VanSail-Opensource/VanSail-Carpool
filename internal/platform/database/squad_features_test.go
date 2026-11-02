package database

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
)

func TestSquadFeaturesRoundTripPreserveAndClear(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	input := SquadProductInput{GroupID: "features-roundtrip", UpstreamPresent: true, Features: squadprofile.Features{
		{Icon: "route", Title: " Direct route ", Detail: " Stable "},
		{Icon: "shield", Title: "Private"},
	}}
	product, err := store.SaveSquadProduct(ctx, input)
	if err != nil || len(product.Features) != 2 || product.Features[0].Title != "Direct route" || product.Features[1].Icon != "shield" {
		t.Fatalf("feature roundtrip: %+v %v", product.Features, err)
	}
	input.Features = nil
	product, err = store.SaveSquadProduct(ctx, input)
	if err != nil || len(product.Features) != 2 || product.Features[0].Detail != "Stable" {
		t.Fatalf("omitted features were not preserved: %+v %v", product.Features, err)
	}
	input.Features = squadprofile.Features{}
	product, err = store.SaveSquadProduct(ctx, input)
	if err != nil || len(product.Features) != 0 {
		t.Fatalf("empty features did not clear: %+v %v", product.Features, err)
	}
	encoded, err := json.Marshal(product)
	if err != nil || !strings.Contains(string(encoded), `"features":[]`) {
		t.Fatalf("empty features not represented as array: %s %v", encoded, err)
	}
}

func TestSquadFeaturesRejectInvalidWithoutChangingExisting(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	input := SquadProductInput{GroupID: "features-invalid", UpstreamPresent: true, Features: squadprofile.Features{{Icon: "globe", Title: "Known"}}}
	if _, err := store.SaveSquadProduct(ctx, input); err != nil {
		t.Fatal(err)
	}
	for _, features := range []squadprofile.Features{
		{{Icon: "javascript:alert(1)", Title: "Unsafe"}},
		{{Icon: "globe", Title: "  "}},
		{{Icon: "globe", Title: strings.Repeat("界", 81)}},
		{{Icon: "globe", Title: "Known", Detail: strings.Repeat("x", 161)}},
		make(squadprofile.Features, 13),
	} {
		input.Features = features
		if _, err := store.SaveSquadProduct(ctx, input); !errors.Is(err, squadprofile.ErrInvalidFeatures) {
			t.Fatalf("invalid feature accepted: %+v %v", features, err)
		}
	}
	product, err := store.SquadProductByGroupID(ctx, input.GroupID)
	if err != nil || len(product.Features) != 1 || product.Features[0].Title != "Known" {
		t.Fatalf("invalid writes changed stored list: %+v %v", product.Features, err)
	}
}
