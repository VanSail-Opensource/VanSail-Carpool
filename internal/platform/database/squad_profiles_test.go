package database

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/squadprofile"
)

func TestSquadProfileOverrideRoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const uuid = "profile-round-trip"
	if _, err := store.ImportSquad(ctx, uuid, "Profile round trip"); err != nil {
		t.Fatal(err)
	}
	profile := &squadprofile.Profile{Type: squadprofile.InternationalNetwork, CountryCode: "sg", UpstreamCarriers: []string{"Carrier A, Carrier B"}}
	product, err := store.SaveSquadProduct(ctx, SquadProductInput{RemnaSquadUUID: uuid, Name: "ignored", Description: "Extra **Markdown**", Profile: profile, UpstreamPresent: true})
	if err != nil {
		t.Fatalf("SaveSquadProduct() error = %v", err)
	}
	if product.Profile == nil || product.Profile.CountryCode != "SG" || len(product.Profile.UpstreamCarriers) != 2 {
		t.Fatalf("saved profile = %+v", product.Profile)
	}
	if product.Description != "Extra **Markdown**" {
		t.Fatalf("description = %q", product.Description)
	}
}

func TestLegacySquadDescriptionKeepsNullableProfile(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	const uuid = "legacy-profile"
	if _, err := store.ImportSquad(ctx, uuid, "Legacy"); err != nil {
		t.Fatal(err)
	}
	product, err := store.SaveSquadProduct(ctx, SquadProductInput{RemnaSquadUUID: uuid, Description: "Legacy Markdown", UpstreamPresent: true})
	if err != nil {
		t.Fatalf("SaveSquadProduct() error = %v", err)
	}
	if product.Profile != nil || product.Description != "Legacy Markdown" {
		t.Fatalf("legacy product = %+v", product)
	}
}
