package app

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s proxyConnectivitySource) loadStandaloneTargets(ctx context.Context, identity xnode.Identity) (connectivity.User, []connectivity.Target, error) {
	user := connectivity.User{ID: identity.ProxyUserID, Username: identity.ServiceName, Status: "ACTIVE"}
	proxies, err := s.provider.native.ServiceProxies(ctx, identity)
	if err != nil {
		return user, nil, err
	}
	targets := []connectivity.Target{}
	for _, proxy := range proxies {
		outbound, err := proxy.Outbound()
		if err != nil {
			return user, nil, err
		}
		raw, err := json.Marshal(outbound)
		if err != nil {
			return user, nil, err
		}
		targets = append(targets, connectivity.Target{HostUUID: proxy.ID, Remark: proxy.Name, Address: proxy.Address, Port: proxy.Port, NativeOutbound: raw})
	}
	if len(targets) == 0 {
		return user, nil, connectivityFailure("ACCOUNT_UNAVAILABLE")
	}
	return user, targets, nil
}
