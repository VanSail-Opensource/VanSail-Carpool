package xnode

import (
	"math"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirewallSnapshotIdentitySurvivesAttemptReset(t *testing.T) {
	const job = "01234567-89ab-4cde-8012-3456789abcde"
	member := FirewallMember{Version: 17, SourceVersion: 23}
	for _, sources := range []bool{false, true} {
		bank := "dynamic"
		if sources {
			bank = "sources"
		}
		t.Run(bank, func(t *testing.T) {
			original := firewallSnapshotID(job, 1, member, sources)
			if original != firewallSnapshotID(job, 1, member, sources) {
				t.Fatal("unchanged delivery lost its replay identity")
			}
			if original == firewallSnapshotID(job, 1, member, !sources) {
				t.Fatal("different banks shared a snapshot identity")
			}
			if original == firewallSnapshotID(job, 2, member, sources) || original == firewallSnapshotID("another-job", 1, member, sources) {
				t.Fatal("another delivery reused the old snapshot")
			}
			next := member
			if sources {
				next.SourceVersion++
			} else {
				next.Version++
			}
			// An administrator can reset attempts to one while an older snapshot
			// is staged. Its ID must change before sending the new version's chunk 0.
			if original == firewallSnapshotID(job, 1, next, sources) {
				t.Fatal("reset attempt reused an unfinished snapshot for another version")
			}
			unrelated := member
			if sources {
				unrelated.Version++
			} else {
				unrelated.SourceVersion++
			}
			if original != firewallSnapshotID(job, 1, unrelated, sources) {
				t.Fatal("another bank's change restarted the unchanged snapshot")
			}
			largest := firewallSnapshotID(job, math.MaxInt, FirewallMember{Version: math.MaxUint64, SourceVersion: math.MaxUint64}, sources)
			if !wire.ValidName(largest) {
				t.Fatalf("snapshot identity exceeds protocol bounds: %q", largest)
			}
		})
	}
}
