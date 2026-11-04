package xnode

import (
	"cmp"
	"math/big"
	"net/netip"
	"slices"
	"time"
)

const maxProfileActiveIPs = 256
const maxProfileIPObservations = 4096

type ProfileIPObservation struct {
	IP           string    `json:"ip"`
	LastActive   time.Time `json:"lastActive"`
	Observations string    `json:"observations"`
}

// The caller supplies captured immutable reports and owner-scoped principals.
// Legacy IP arrays have no capture times and cannot claim fresh activity.
func profileNodeActivity(item *UserNodeMetric, snapshot liveSnapshot, peer *agentPeer, principals []int64, now time.Time, budgets ...int) {
	item.ActiveIPStatus, item.SampledAt, item.ActiveIPsTruncated = "unavailable", nil, false
	item.ActiveIPs = []ProfileIPObservation{}
	if snapshot.report.Users.SampledAt != nil {
		at := *snapshot.report.Users.SampledAt
		item.SampledAt = &at
	}
	if peer == nil || !peer.activityReports || item.SampledAt == nil {
		return
	}
	if !item.Available {
		item.ActiveIPStatus = "stale"
		return
	}
	item.ActiveIPStatus = "available"
	owned := map[int64]bool{}
	for _, principal := range principals {
		owned[principal] = true
	}
	limit := maxProfileActiveIPs
	if len(budgets) > 0 {
		limit = min(limit, max(0, budgets[0]))
	}
	item.ActiveIPsTruncated = snapshot.report.Users.ObservationDrops > 0
	if limit == 0 {
		for _, observation := range snapshot.report.Users.ActiveIPObservations {
			if owned[int64(observation.UserID)] && !observation.LastActive.Before(now.Add(-2*time.Hour)) {
				if _, err := netip.ParseAddr(observation.IP); err == nil {
					item.ActiveIPsTruncated = true
					break
				}
			}
		}
		return
	}
	byIP := map[string]*ProfileIPObservation{}
	for _, observation := range snapshot.report.Users.ActiveIPObservations {
		if !owned[int64(observation.UserID)] || observation.LastActive.Before(now.Add(-2*time.Hour)) {
			continue
		}
		address, err := netip.ParseAddr(observation.IP)
		if err != nil {
			continue
		}
		ip := address.Unmap().String()
		value := byIP[ip]
		if value == nil {
			value = &ProfileIPObservation{IP: ip, Observations: "0"}
			byIP[ip] = value
		}
		if observation.LastActive.After(value.LastActive) {
			value.LastActive = observation.LastActive
		}
		count, _ := new(big.Int).SetString(value.Observations, 10)
		value.Observations = count.Add(count, new(big.Int).SetUint64(observation.Observations)).String()
	}
	for _, value := range byIP {
		item.ActiveIPs = append(item.ActiveIPs, *value)
	}
	slices.SortFunc(item.ActiveIPs, func(a, b ProfileIPObservation) int {
		return cmp.Or(b.LastActive.Compare(a.LastActive), cmp.Compare(a.IP, b.IP))
	})
	if len(item.ActiveIPs) > limit {
		item.ActiveIPsTruncated = true
		// Copy the retained rows so a clipped slice does not keep the complete
		// node history and its strings alive until response encoding finishes.
		item.ActiveIPs = slices.Clone(item.ActiveIPs[:limit])
	}
}
