package xnode

import wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"

func filterPurgeMetrics(users wire.UserMetrics, owned map[int]bool) wire.UserMetrics {
	users.Connections = withoutPurgeKeys(users.Connections, owned)
	users.TotalConnections = withoutPurgeKeys(users.TotalConnections, owned)
	users.ActiveIPs = withoutPurgeKeys(users.ActiveIPs, owned)
	users.Status = withoutPurgeKeys(users.Status, owned)
	users.Speed = withoutPurgeKeys(users.Speed, owned)
	users.Rates = withoutPurgeKeys(users.Rates, owned)
	for _, ip := range users.ActiveIPObservations {
		if owned[ip.UserID] {
			retained := make([]wire.IPActivity, 0, len(users.ActiveIPObservations))
			for _, item := range users.ActiveIPObservations {
				if !owned[item.UserID] {
					retained = append(retained, item)
				}
			}
			users.ActiveIPObservations = retained
			break
		}
	}
	return users
}

func withoutPurgeKeys[T any](items map[int]T, owned map[int]bool) map[int]T {
	for id := range items {
		if owned[id] {
			retained := make(map[int]T, len(items))
			for key, value := range items {
				if !owned[key] {
					retained[key] = value
				}
			}
			return retained
		}
	}
	return items
}
