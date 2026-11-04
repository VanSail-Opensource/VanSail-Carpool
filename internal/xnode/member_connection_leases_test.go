package xnode

import (
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"net/netip"
	"testing"
	"time"
)

func TestManualLeasePreservesDeadlineAndOwnerIsolation(t *testing.T) {
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	m := newAutomaticLeaseManager("manual")
	first, err := m.manualGrant(10, "8.8.8.8", now)
	if err != nil || first.LastObservedAt != nil {
		t.Fatal(first, err)
	}
	duplicate, err := m.manualGrant(10, "8.8.8.8", now.Add(time.Hour))
	if err != nil || !duplicate.ExpiresAt.Equal(*first.ExpiresAt) || !duplicate.InitialLeaseAt.Equal(*first.InitialLeaseAt) {
		t.Fatal("duplicate extended grant", duplicate, err)
	}
	other, _ := m.memberLeases([]int64{20}, now)
	if len(other) != 0 {
		t.Fatal("foreign grant leaked", other)
	}
	shared, err := m.manualGrant(20, "8.8.8.8", now.Add(2*time.Hour))
	if err != nil || !shared.ExpiresAt.Equal(*first.ExpiresAt) {
		t.Fatal(shared, err)
	}
	// Actual owner observations are separate from the fleet last-active time.
	at := now.Add(13 * time.Hour)
	m.observe([]AutomaticLeaseObservation{{"8.8.8.8", "node", at}}, at, time.Minute)
	item := m.leases[netip.MustParseAddr("8.8.8.8")]
	item.owners[10] = at
	owners, _ := m.memberLeases([]int64{10, 20}, at)
	if len(owners) != 1 || !owners[0].InitialLeaseAt.Equal(now) || !owners[0].LastObservedAt.Equal(at) {
		t.Fatal(owners)
	}
	manual, _ := m.memberLeases([]int64{20}, at)
	if len(manual) != 1 || manual[0].LastObservedAt != nil {
		t.Fatal("foreign observation leaked", manual)
	}
	if item.observations != 1 || !item.first.Equal(now) {
		t.Fatal(item)
	}
}

func TestManualLeasePublicCanonicalValidationAndDisabledAutomation(t *testing.T) {
	now := time.Now().UTC()
	for _, ip := range []string{"127.0.0.1", "192.168.1.1", "::1", "8.8.8.8/32", "8.8.8.8 ", "2001:4860:0000::8888", "fe80::1%eth0", "::ffff:8.8.8.8"} {
		t.Run(ip, func(t *testing.T) {
			if _, err := newAutomaticLeaseManager("invalid").manualGrant(10, ip, now); err == nil {
				t.Fatal("accepted", ip)
			}
		})
	}
	m := newAutomaticLeaseManager("ipv6")
	if _, err := m.manualGrant(10, "2001:4860::8888", now); err != nil {
		t.Fatal(err)
	}
	m.configure(AutomaticWhitelistConfig{Enabled: false, DurationHours: 24})
	if _, err := m.manualGrant(10, "8.8.8.8", now); err == nil {
		t.Fatal("disabled grant accepted")
	}
	items, _ := m.memberLeases([]int64{10}, now.Add(25*time.Hour))
	if len(items) != 0 || m.ownerAssociations != 0 {
		t.Fatal("expired ownership retained", items, m.ownerAssociations)
	}
}

func TestObservedOwnershipIsBoundedAndDeduplicated(t *testing.T) {
	now := time.Now().UTC()
	m := newAutomaticLeaseManager("observed")
	m.observe([]AutomaticLeaseObservation{{"8.8.8.8", "node", now}}, now, time.Minute)
	report := wire.UserMetrics{ActiveIPObservations: []wire.IPActivity{{UserID: 10, IP: "8.8.8.8", LastActive: now}, {UserID: 11, IP: "8.8.8.8", LastActive: now}}}
	m.associate(report, now, time.Minute)
	m.associate(report, now, time.Minute)
	if m.ownerAssociations != 2 {
		t.Fatal("duplicate owner association")
	}
	items, _ := m.memberLeases([]int64{10, 11}, now)
	if len(items) != 1 {
		t.Fatal(items)
	}
	m.expire(now.Add(25 * time.Hour))
	if m.ownerAssociations != 0 {
		t.Fatal("association capacity leaked")
	}
}
