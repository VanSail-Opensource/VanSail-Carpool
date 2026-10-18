package app

import (
	"context"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type disabledMonitorRepository struct{ xnode.Repository }

func (disabledMonitorRepository) XNodeIdentity(context.Context, int64) (xnode.Identity, error) {
	return xnode.Identity{ProxyUserID: 42, ServiceName: "disabled-monitor", AccessDisabled: true}, nil
}

func TestNativeMonitoringRejectsDisabledIdentityBeforeResolvingHosts(t *testing.T) {
	source := proxyConnectivitySource{provider: proxyProvider{native: xnode.NewService(disabledMonitorRepository{}, nil, nil, nil, nil)}}
	if err := source.Validate(context.Background(), 42); connectivity.ErrorCode(err) != "CONNECTIVITY_ACCOUNT_UNAVAILABLE" {
		t.Fatal(err)
	}
	if _, targets, err := source.Load(context.Background(), 42); connectivity.ErrorCode(err) != "CONNECTIVITY_ACCOUNT_UNAVAILABLE" || len(targets) > 0 {
		t.Fatal("disabled identity exposed probe credentials", targets, err)
	}
}
