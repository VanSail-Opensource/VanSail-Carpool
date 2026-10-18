package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (a proxyProvider) DashboardForMember(ctx context.Context, user model.User) (catalog.RemoteDashboard, error) {
	identity, err := a.native.IdentityForUser(ctx, user.ID)
	if err == nil {
		return a.Dashboard(ctx, strconv.FormatInt(identity.ProxyUserID, 10))
	}
	if !errors.Is(err, xnode.ErrNotFound) {
		return catalog.RemoteDashboard{}, err
	}
	bridge, err := a.legacy(ctx)
	if err != nil {
		return catalog.RemoteDashboard{}, err
	}
	if bridge && user.RemnaUserID != nil {
		return a.remnaAdapter.Dashboard(ctx, *user.RemnaUserID)
	}
	return catalog.RemoteDashboard{}, xnode.ErrNotFound
}
