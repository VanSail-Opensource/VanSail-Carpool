package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestPAYGPreparationRotatesOnlyProvenUnpublishedOrphan(t *testing.T) {
	for _, exposed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unpublished", true: "accounting_pending"}[exposed], func(t *testing.T) {
			s, input := paygPreparationFixture(t)
			ctx := context.Background()
			identity, err := s.SaveXNodeIdentity(ctx, xnode.Identity{ServiceName: input.ServiceName, CredentialsCipher: "abandoned-old-cipher"})
			if err != nil {
				t.Fatal(err)
			}
			old := xnode.Period{ID: "payg-unassociated", ProxyUserID: identity.ProxyUserID, StartedAt: time.Now().UTC(), Strategy: "PAYG", Status: "open"}
			if err = s.SaveXNodePeriod(ctx, old); err != nil {
				t.Fatal(err)
			}
			if exposed {
				if err = s.TrackXNodePeriodNode(ctx, old.ID, "node"); err != nil {
					t.Fatal(err)
				}
			}
			account, err := s.PrepareXNodePAYGAccount(ctx, input)
			stored, e := s.XNodeIdentity(ctx, identity.ProxyUserID)
			if e != nil {
				t.Fatal(e)
			}
			if exposed {
				if !errors.Is(err, xnode.ErrAccessAccountingPending) || stored.CredentialsCipher != "abandoned-old-cipher" {
					t.Fatal("uncertain preparation changed", stored, err)
				}
				var count int
				if e = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_payg_accounts WHERE proxy_user_id=$1`, identity.ProxyUserID).Scan(&count); e != nil || count != 0 {
					t.Fatal("uncertain participant published", count, e)
				}
			} else if err != nil || account.ProxyUserID != identity.ProxyUserID || account.PeriodID != input.PeriodID || stored.CredentialsCipher != input.CredentialsCipher {
				t.Fatal("known unpublished orphan was not rotated atomically", account, stored, err)
			}
			retained, e := s.XNodePeriod(ctx, old.ID)
			if e != nil || retained.UsedBytes != 0 || retained.Status != "open" {
				t.Fatal("prior accounting fact was erased", retained, e)
			}
		})
	}
}
