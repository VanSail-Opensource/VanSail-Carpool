package catalog

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type nativeDashboardRemote struct {
	*catalogRemnawave
	owners []string
}

func (r *nativeDashboardRemote) DashboardForMember(_ context.Context, user model.User) (RemoteDashboard, error) {
	r.owners = append(r.owners, user.ID)
	return RemoteDashboard{Statistics: model.Statistics{UsedTrafficBytes: "42"}, SubscriptionURL: "https://panel.example/native/" + user.ID}, nil
}

func TestNativeDashboardUsesOwnerWithoutLegacyIdentity(t *testing.T) {
	remote := &nativeDashboardRemote{catalogRemnawave: &catalogRemnawave{}}
	service := NewService(&catalogRepository{}, remote, time.Minute)
	for _, id := range []string{"member", "member", "other"} {
		value, err := service.Dashboard(context.Background(), model.User{ID: id})
		if err != nil || value.Statistics == nil || value.Statistics.UsedTrafficBytes != "42" || value.SubscriptionURL == nil || *value.SubscriptionURL != "https://panel.example/native/"+id {
			t.Fatal(value, err)
		}
	}
	if len(remote.owners) != 2 || remote.owners[0] != "member" || remote.owners[1] != "other" {
		t.Fatal("native cache crossed ownership", remote.owners)
	}
}
