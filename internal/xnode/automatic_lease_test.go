package xnode

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAutomaticLeaseHalfRenewalExpiryAndConfiguration(t *testing.T) {
	m := newAutomaticLeaseManager("master")
	at := time.Now().UTC()
	observe := func(ip, node string, when time.Time) bool {
		return m.observe([]AutomaticLeaseObservation{{ip, node, when}}, when, time.Minute)
	}
	if !observe("::ffff:8.8.8.8", "node", at) {
		t.Fatal("canonical public address was not leased")
	}
	for _, when := range []time.Time{at, at.Add(12*time.Hour - time.Nanosecond)} {
		if observe("8.8.8.8", "node", when) {
			t.Fatal("duplicate or early observation renewed the grant")
		}
	}
	first, _, _ := m.delivery(0, at.Add(12*time.Hour-time.Nanosecond))
	if len(first) != 1 || !first[0].Until.Equal(at.Add(24*time.Hour)) {
		t.Fatal("initial duration changed", first)
	}
	half := at.Add(12 * time.Hour)
	if !observe("8.8.8.8", "node", half) {
		t.Fatal("exact halfway observation failed to renew")
	}
	items, cursor, _ := m.delivery(first[0].Generation, half)
	if len(items) != 1 || !items[0].Until.Equal(half.Add(48*time.Hour)) {
		t.Fatal("renewal did not use twice configured duration", items)
	}
	if observe("8.8.8.8", "node", half.Add(24*time.Hour-time.Nanosecond)) {
		t.Fatal("renewed interval's halfway boundary was shortened")
	}
	m.configure(AutomaticWhitelistConfig{true, 6})
	if !observe("8.8.8.8", "node", half.Add(24*time.Hour)) {
		t.Fatal("new duration ignored at next qualifying renewal")
	}
	items, _, _ = m.delivery(cursor, half.Add(24*time.Hour))
	if len(items) != 1 || !items[0].Until.Equal(half.Add(36*time.Hour)) {
		t.Fatal("renewal compounded or ignored new setting", items)
	}
	m.expire(half.Add(36 * time.Hour))
	if len(m.leases)+len(m.expiry)+len(m.pendingBans)+len(m.snapshot) != 0 {
		t.Fatal("expired lease retained registry or delivery state")
	}
	if grants, _, _ := m.delivery(0, half.Add(36*time.Hour)); len(grants) != 0 {
		t.Fatal("expiry generated another command")
	}
}

func TestAutomaticLeaseSourcesReplayBoundsAndPrivateViews(t *testing.T) {
	m := newAutomaticLeaseManager("master")
	at := time.Now().UTC()
	var wg sync.WaitGroup
	for i := range 12 {
		wg.Go(func() {
			for range 3 {
				m.observe([]AutomaticLeaseObservation{{"2001:4860::8", fmt.Sprint(i), at}}, at, time.Minute)
			}
		})
	}
	wg.Wait()
	result, err := m.page("2001:4860", 1, 10, []uint64{0, 1}, at)
	if err != nil || len(result.Items) != 1 || result.Items[0].SourceCount != maximumLeaseSources || !result.Items[0].SourceCountTruncated || result.Items[0].PendingNodes != 1 || m.generation != 1 {
		t.Fatal("duplicate grants, unbounded source metadata or incorrect delivery", result, err)
	}
	if result.Items[0].ObservationCount != maximumLeaseSources {
		t.Fatal("replayed or saturated sources inflated exact observations", result.Items[0])
	}
	result.Items[0].SourceNodeIDs[0] = "modified"
	grants, _, _ := m.delivery(0, at)
	grants[0].IP = "modified"
	again, _, _ := m.delivery(0, at)
	if again[0].IP != "2001:4860::8" {
		t.Fatal("delivery payload aliases shared snapshot")
	}
	for _, ip := range []string{"127.0.0.1", "192.168.1.1", "100.64.0.1", "2001:db8::1", "invalid", "fe80::1%eth0"} {
		m.observe([]AutomaticLeaseObservation{{ip, "node", at}}, at, time.Minute)
	}
	if len(m.leases) != 1 {
		t.Fatal("non-public addresses leased")
	}
	m.configure(AutomaticWhitelistConfig{false, 24})
	if m.observe([]AutomaticLeaseObservation{{"8.8.8.8", "node", at}}, at, time.Minute) {
		t.Fatal("disabled manager granted a lease")
	}
}

func TestDisabledLeaseCreationPreservesUnexpiredSynchronizationAndStatistics(t *testing.T) {
	m := newAutomaticLeaseManager("master")
	at := time.Now().UTC()
	m.observe([]AutomaticLeaseObservation{{"2606:4700:4700::1111", "one", at}}, at, time.Minute)
	m.configure(AutomaticWhitelistConfig{false, 6})
	later := at.Add(12 * time.Hour)
	if m.observe([]AutomaticLeaseObservation{{"2606:4700:4700::1111", "two", later}}, later, time.Minute) {
		t.Fatal("disabled creation renewed an existing deadline")
	}
	grants, _, _ := m.delivery(0, later)
	if len(grants) != 1 || !grants[0].Until.Equal(at.Add(24*time.Hour)) {
		t.Fatal("configuration change removed an unexpired reconnect grant", grants)
	}
	page, err := m.page("2606:4700:4700:0:0:0:0:1111", 1, 25, nil, later)
	if err != nil || len(page.Items) != 1 || !page.Items[0].LastActive.Equal(later) || page.Items[0].ObservationCount != 2 {
		t.Fatal("canonical search or ongoing activity was lost", page, err)
	}
}
