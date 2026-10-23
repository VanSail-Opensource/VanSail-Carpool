package app

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"strconv"
)

type proxyConnectivitySource struct{ provider proxyProvider }

func (s proxyConnectivitySource) Resolve(ctx context.Context, username string) (connectivity.User, error) {
	remote, exists, err := s.provider.FindUserByUsername(ctx, username)
	if err != nil || !exists {
		return connectivity.User{}, connectivityFailure("ACCOUNT_UNAVAILABLE")
	}
	id, err := strconv.ParseInt(remote.ID, 10, 64)
	if err != nil {
		return connectivity.User{}, err
	}
	if err = s.Validate(ctx, id); err != nil {
		return connectivity.User{}, err
	}
	return connectivity.User{ID: id, Username: remote.Username, Status: "ACTIVE"}, nil
}
func (s proxyConnectivitySource) Validate(ctx context.Context, id int64) error {
	identity, err := s.provider.native.Identity(ctx, strconv.FormatInt(id, 10))
	if err != nil {
		return err
	}
	if identity.AccessDisabled {
		return connectivityFailure("ACCOUNT_UNAVAILABLE")
	}
	if identity.ServiceName != "" {
		access, err := s.provider.native.ServiceAccess(ctx, identity)
		if err != nil {
			return err
		}
		if access != nil {
			proxies, err := s.provider.native.ServiceProxies(ctx, identity)
			if err != nil {
				return err
			}
			if len(proxies) == 0 {
				return connectivityFailure("ACCOUNT_UNAVAILABLE")
			}
			return nil
		}
		_, _, err = s.Load(ctx, id)
		return err
	}
	proxies, err := s.provider.native.MemberProxies(ctx, identity.UserID)
	if err != nil {
		return err
	}
	if len(proxies) == 0 {
		return connectivityFailure("ACCOUNT_UNAVAILABLE")
	}
	return nil
}
func (s proxyConnectivitySource) Load(ctx context.Context, id int64) (connectivity.User, []connectivity.Target, error) {
	identity, err := s.provider.native.Identity(ctx, strconv.FormatInt(id, 10))
	if err != nil {
		return connectivity.User{}, nil, err
	}
	user := connectivity.User{ID: id, Username: identity.Username, Status: "ACTIVE"}
	if identity.AccessDisabled {
		return user, nil, connectivityFailure("ACCOUNT_UNAVAILABLE")
	}
	targets := []connectivity.Target{}
	if identity.ServiceName != "" {
		user.Username = identity.ServiceName
		access, err := s.provider.native.ServiceAccess(ctx, identity)
		if err != nil {
			return user, nil, err
		}
		if access != nil {
			return s.loadStandaloneTargets(ctx, identity)
		}
		endpoints, err := s.provider.native.Endpoints(ctx)
		if err != nil {
			return user, nil, err
		}
		bindings, err := s.provider.store.XNodeServiceBindings(ctx, id)
		if err != nil {
			return user, nil, err
		}
		for _, endpoint := range endpoints {
			if !endpoint.Enabled {
				continue
			}
			node, err := s.provider.store.XNodeNode(ctx, endpoint.NodeID)
			if err != nil {
				return user, nil, err
			}
			if !node.Enabled {
				continue
			}
			for _, binding := range bindings {
				if binding.NodeID == endpoint.NodeID && binding.InboundTag == endpoint.InboundTag && binding.ValidUntil.After(s.provider.native.Now()) {
					proxy, err := s.provider.native.ServiceProxy(ctx, identity, endpoint)
					if err != nil {
						return user, nil, err
					}
					out, err := proxy.Outbound()
					if err != nil {
						return user, nil, err
					}
					raw, err := json.Marshal(out)
					if err != nil {
						return user, nil, err
					}
					targets = append(targets, connectivity.Target{HostUUID: endpoint.ID, Remark: endpoint.Name, Address: endpoint.Address, Port: endpoint.Port, NativeOutbound: raw})
					break
				}
			}
		}
	} else {
		proxies, err := s.provider.native.MemberProxies(ctx, identity.UserID)
		if err != nil {
			return user, nil, err
		}
		for _, proxy := range proxies {
			out, err := proxy.Outbound()
			if err != nil {
				return user, nil, err
			}
			raw, err := json.Marshal(out)
			if err != nil {
				return user, nil, err
			}
			targets = append(targets, connectivity.Target{HostUUID: proxy.ID, Remark: proxy.Name, Address: proxy.Address, Port: proxy.Port, NativeOutbound: raw})
		}
	}
	if len(targets) == 0 {
		return user, nil, connectivityFailure("ACCOUNT_UNAVAILABLE")
	}
	return user, targets, nil
}
