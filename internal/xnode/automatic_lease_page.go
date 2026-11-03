package xnode

import (
	"slices"
	"strings"
	"time"
)

func (m *automaticLeaseManager) page(search string, page, size int, watermarks []uint64, now time.Time) (AutomaticLeasePage, error) {
	result := AutomaticLeasePage{Items: []AutomaticLeaseView{}, Page: page, PageSize: size, FetchedAt: now}
	if !validAutomaticLeasePage(search, page, size) {
		return result, ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expireLocked(now)
	result.Settings = m.config
	result.Statistics = AutomaticLeaseStatistics{Active: len(m.leases), Created: m.created, Renewed: m.renewed, Expired: m.expired, Rejected: m.rejected}
	search = strings.ToLower(strings.TrimSpace(search))
	items := make([]*automaticLease, 0, len(m.leases))
	for _, item := range m.leases {
		if item.ip.Is4() {
			result.Statistics.IPv4++
		} else {
			result.Statistics.IPv6++
		}
		if search == "" || strings.Contains(item.ip.String(), search) {
			items = append(items, item)
		}
	}
	slices.SortFunc(items, func(a, b *automaticLease) int { return a.ip.Compare(b.ip) })
	watermarks = slices.Clone(watermarks)
	slices.Sort(watermarks)
	result.Total = len(items)
	start := min((page-1)*size, len(items))
	for _, item := range items[start:min(start+size, len(items))] {
		pending, _ := slices.BinarySearch(watermarks, item.generation)
		view := AutomaticLeaseView{IP: item.ip.String(), FirstActive: item.first, LastActive: item.last, LeaseStarted: item.started, RenewAfter: item.started.Add(item.until.Sub(item.started) / 2), ExpiresAt: item.until, RemainingSeconds: max(0, int64(item.until.Sub(now)/time.Second)), ObservationCount: item.observations, RenewalCount: item.renewals, SourceCount: item.sourceCount, SourceCountTruncated: item.sourcesTruncated, SourceNodeIDs: []string{}, PendingNodes: pending, BanReleasePending: m.pendingBans[item.ip] != 0}
		if item.renewed != nil {
			at := *item.renewed
			view.RenewedAt = &at
		}
		for _, source := range item.sources[:item.sourceCount] {
			view.SourceNodeIDs = append(view.SourceNodeIDs, source.node)
		}
		result.Items = append(result.Items, view)
	}
	return result, nil
}

func validAutomaticLeasePage(search string, page, size int) bool {
	return page >= 1 && page <= maximumAutomaticLeases && size >= 1 && size <= 100 && len(search) <= 128
}
