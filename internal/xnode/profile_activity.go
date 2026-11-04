package xnode

import (
	"cmp"
	"math/big"
	"slices"
	"time"
)

const maxProfileActiveIPs = 256

type ProfileIPObservation struct {
	IP           string    `json:"ip"`
	LastActive   time.Time `json:"lastActive"`
	Observations string    `json:"observations"`
}

// The caller owns the master read lock and supplies only owner-scoped principals.
// Legacy IP arrays have no capture times and cannot claim fresh activity.
func profileNodeActivity(item *UserNodeMetric, snapshot liveSnapshot, peer *agentPeer, principals []int64, now time.Time) {
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
	byIP := map[string]*ProfileIPObservation{}
	for _, observation := range snapshot.report.Users.ActiveIPObservations {
		if !owned[int64(observation.UserID)] || observation.LastActive.Before(now.Add(-2*time.Hour)) {
			continue
		}
		ip, err := FirewallPublicIP(observation.IP)
		if err != nil {
			continue
		}
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
	item.ActiveIPsTruncated = len(item.ActiveIPs) > maxProfileActiveIPs || snapshot.report.Users.ObservationDrops > 0
	item.ActiveIPs = item.ActiveIPs[:min(len(item.ActiveIPs), maxProfileActiveIPs)]
}
