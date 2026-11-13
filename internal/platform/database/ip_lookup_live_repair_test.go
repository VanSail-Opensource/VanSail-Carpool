package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/iplookup"
)

func liveRepairFixture(t *testing.T) (*Store, string, time.Time) {
	t.Helper()
	store := newTestStore(t)
	at := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	details := iplookup.LiveDetails{IP: "1.1.1.1", FetchedAt: stamp(at),
		Topology: iplookup.BGPTopology{LiveMeta: iplookup.LiveMeta{Status: "empty"}, ProjectionVersion: 2},
		PTR:      iplookup.PTRDetails{LiveMeta: iplookup.LiveMeta{Status: "error"}},
		Shodan:   iplookup.ShodanDetails{LiveMeta: iplookup.LiveMeta{Status: "error"}}}
	id, err := store.SaveIPDetails(context.Background(), "", details, at)
	if err != nil || id == "" {
		t.Fatal("save partial snapshot", id, err)
	}
	return store, id, at
}

func loadRepairLive(t *testing.T, store *Store, id string) iplookup.LiveDetails {
	t.Helper()
	details, _, err := store.IPLiveSnapshotByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return details
}

func TestConcurrentLiveRepairsCannotLoseCompletedSections(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, id, completed := liveRepairFixture(t)
	first, second := loadRepairLive(t, store, id), loadRepairLive(t, store, id)
	first.PTR.Status, first.PTR.Domains = "success", []string{"retained.example"}
	second.Shodan.Status, second.Shodan.Ports = "success", []int{443}
	var revisions [2]string
	errs := runTogether(func() error {
		var err error
		revisions[0], err = store.UpdateIPLiveSnapshot(ctx, id, first)
		return err
	}, func() error {
		var err error
		revisions[1], err = NewStore(store.DB()).UpdateIPLiveSnapshot(ctx, id, second)
		return err
	})
	winners, conflicts := 0, 0
	for _, err := range errs {
		if err == nil {
			winners++
		} else if errors.Is(err, iplookup.ErrRepairConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	current := loadRepairLive(t, store, id)
	if winners != 1 || conflicts != 1 || current.RepairRevision == first.RepairRevision {
		t.Fatal("live repairs did not use one document revision", winners, conflicts, revisions)
	}
	if revisions[0] != "" {
		if current.RepairRevision != revisions[0] || current.PTR.Status != "success" ||
			len(current.PTR.Domains) != 1 || current.PTR.Domains[0] != "retained.example" || current.Shodan.Status != "error" {
			t.Fatal("losing Shodan repair erased completed PTR evidence", current)
		}
	} else if current.RepairRevision != revisions[1] || current.Shodan.Status != "success" ||
		len(current.Shodan.Ports) != 1 || current.Shodan.Ports[0] != 443 || current.PTR.Status != "error" {
		t.Fatal("losing PTR repair erased completed Shodan evidence", current)
	}
	if _, err := store.UpdateIPLiveSnapshot(ctx, id, second); !errors.Is(err, iplookup.ErrRepairConflict) {
		t.Fatal("stale partial snapshot overwrote winner", err)
	}
	current.PTR.Status, current.Shodan.Status = "success", "success"
	if _, err := store.UpdateIPLiveSnapshot(ctx, id, current); err != nil {
		t.Fatal("fresh snapshot could not finish remaining section", err)
	}
	var at, status string
	if err := store.DB().QueryRowContext(ctx, `SELECT completed_at,status FROM ip_lookup_live_snapshots WHERE id=$1`, id).Scan(&at, &status); err != nil || at != stamp(completed) || status != "succeeded" {
		t.Fatal("repair changed freeze ordering", at, status, err)
	}
}

func TestLiveRepairCannotChangeSnapshotAddressOrFetchTime(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, id, _ := liveRepairFixture(t)
	original := loadRepairLive(t, store, id)
	for _, change := range []func(*iplookup.LiveDetails){
		func(r *iplookup.LiveDetails) { r.SnapshotID = "another-snapshot" },
		func(r *iplookup.LiveDetails) { r.IP = "8.8.8.8" },
		func(r *iplookup.LiveDetails) { r.SourceIP = "8.8.8.8" },
		func(r *iplookup.LiveDetails) { r.FetchedAt = stamp(time.Now().UTC()) },
		func(r *iplookup.LiveDetails) { r.RepairRevision = "" },
	} {
		candidate := loadRepairLive(t, store, id)
		change(&candidate)
		if _, err := store.UpdateIPLiveSnapshot(ctx, id, candidate); !errors.Is(err, iplookup.ErrRepairConflict) {
			t.Fatal("repair changed immutable snapshot identity", err)
		}
		if current := loadRepairLive(t, store, id); current.RepairRevision != original.RepairRevision {
			t.Fatal("rejected repair changed retained evidence")
		}
	}
}
