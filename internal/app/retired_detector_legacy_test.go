package app

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
)

func TestRetiredDetectorLegacyReleaseRechecksQueuedManualBanAndPaidTerm(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"paid", "no paid access", "manual ban while queued"} {
		t.Run(mode, func(t *testing.T) {
			queue, err := upstreamqueue.New(upstreamqueue.Config{Name: "retired-detector", Capacity: 2})
			if err != nil {
				t.Fatal(err)
			}
			if err := queue.Start(context.Background()); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = queue.Shutdown(context.Background()) }()
			repo := &detectorLegacyRepoStub{}
			if mode == "paid" {
				repo.purchase = &model.Purchase{TrafficLimitBytes: 123456, ResetStrategy: "MONTH_ROLLING", ValidUntil: time.Now().Add(time.Hour), SquadUUIDs: []string{"enabled", "disabled"}}
			}
			client := &entitlementUpdateClient{}
			adapter := remnaAdapter{queue: queue, clientFactory: func(context.Context) (remnaClient, error) {
				// Runs only after queue admission, after any previous ownership check.
				repo.manual = mode == "manual ban while queued"
				return client, nil
			}}
			if err := releaseRetiredDetectorLegacy(context.Background(), adapter, repo, "member", "7"); err != nil {
				t.Fatal(err)
			}
			if repo.manual {
				if len(client.updates) != 0 {
					t.Fatal("queued release overwrote a manual ban")
				}
				return
			}
			if len(client.updates) != 1 {
				t.Fatal("release must make one status/term write, without resetting traffic")
			}
			got := client.updates[0]
			if got.ID != 7 || got.Status == nil || got.TrafficLimitBytes == nil || got.ActiveInternalSquads == nil {
				t.Fatalf("incomplete update=%+v", got)
			}
			if repo.purchase == nil {
				if *got.Status != remnawave.UserStatusDisabled || *got.TrafficLimitBytes != 0 || len(*got.ActiveInternalSquads) != 0 {
					t.Fatal("registration granted access")
				}
			} else if *got.Status != remnawave.UserStatusActive || *got.TrafficLimitBytes != 123456 || got.ExpireAt == nil || !got.ExpireAt.Equal(repo.purchase.ValidUntil) || got.TrafficLimitStrategy == nil || string(*got.TrafficLimitStrategy) != "MONTH_ROLLING" || len(*got.ActiveInternalSquads) != 1 || (*got.ActiveInternalSquads)[0] != "enabled" {
				t.Fatalf("paid term or preferences changed=%+v", got)
			}
		})
	}
}

type detectorLegacyRepoStub struct {
	manual   bool
	purchase *model.Purchase
}

func (r *detectorLegacyRepoStub) ActiveAdminTemporaryBan(context.Context, string) (*database.AdminTemporaryBan, error) {
	if r.manual {
		return &database.AdminTemporaryBan{}, nil
	}
	return nil, nil
}
func (r *detectorLegacyRepoStub) DesiredEntitlement(context.Context, string, time.Time) (*model.Purchase, error) {
	return r.purchase, nil
}
func (*detectorLegacyRepoStub) EnabledSquadsForRemote(context.Context, string, []string) ([]string, error) {
	return []string{"enabled"}, nil
}
