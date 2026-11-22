package xnode

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestInventoryProjectionPreservesComboAccessAndPAYGPrecedence(t *testing.T) {
	s, r := inventoryProjectionFixture(t)
	ctx := context.Background()
	r.purchase = &model.Purchase{ID: "purchase", UserID: "member", Status: "active", SquadUUIDs: []string{"payg-a"}, ValidFrom: time.Now().Add(-time.Hour), ValidUntil: time.Now().Add(time.Hour)}
	r.period = Period{ID: "period", PurchaseID: "purchase", Status: "open", LimitBytes: 100}
	identity := r.identities[1]
	identity.CurrentPeriodID = "period"
	var err error
	identity.CredentialsCipher, err = sealCredentials(s.vault, identity, Credentials{UUID: "00000000-0000-4000-8000-000000000001"})
	if err != nil {
		t.Fatal(err)
	}
	r.identities[1] = identity
	for _, step := range []struct {
		name   string
		change func()
		combo  int
	}{
		{"paid", func() {}, 8},
		{"quota exhausted", func() { r.period.UsedBytes = 100 }, 0},
		{"quota restored", func() { r.period.UsedBytes = 0 }, 8},
		{"disabled squad", func() { r.disabled = []string{"payg-a"} }, 0},
		{"restored squad", func() { r.disabled = nil }, 8},
		{"unprepared successor", func() { r.period.PurchaseID = "previous" }, 0},
	} {
		t.Run(step.name, func(t *testing.T) {
			step.change()
			before := r.groupReads
			proxies, err := s.MemberProxies(ctx, "member")
			if err != nil || len(proxies) != 16+step.combo {
				t.Fatal("combo eligibility changed other principals", len(proxies), err)
			}
			combo := 0
			for _, proxy := range proxies {
				if proxy.PrincipalID == 2 {
					t.Fatal("covered PAYG principal returned beside paid combo")
				}
				if proxy.PrincipalID == 0 {
					combo++
				}
			}
			if combo != step.combo || r.groupReads != before+1 {
				t.Fatalf("combo=%d groups read=%d", combo, r.groupReads-before)
			}
		})
	}
}
