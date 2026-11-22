package xnode

import (
	"crypto/sha256"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestFirstSyncCompiledProofSkipsReadyOwnersAndResumesAfterMaterialReset(t *testing.T) {
	config, reads := firstSyncReadFixture()
	confirmedAt := time.Now().UTC()
	owner := reads.byUser["owner"]
	owner.FirstSyncedAt = &confirmedAt
	reads.byUser["owner"], reads.byID[owner.ProxyUserID] = owner, owner
	subjects, err := compiledFirstSyncSubjects(config, reads)
	if err != nil || len(subjects) != 0 {
		t.Fatal("already-proven owner repeated first-sync work", subjects, err)
	}
	if evidence := firstSyncCandidates(config, 7, confirmedAt, subjects); evidence != nil {
		t.Fatal("empty proof inventory retained an acknowledgment candidate")
	}
	// The authoritative reset clears readiness and changes the compile epoch.
	// Recompilation must capture the replacement owner material and generation.
	owner.FirstSyncedAt = nil
	owner.CredentialsCipher = "replacement-owner-material"
	reads.byUser["owner"], reads.byID[owner.ProxyUserID] = owner, owner
	account := reads.users["owner"]
	account.AccountGeneration++
	reads.users["owner"] = account
	subjects, err = compiledFirstSyncSubjects(config, reads)
	if err != nil || len(subjects) != 2 {
		t.Fatal("replacement owner lost required applied proof", subjects, err)
	}
	for _, subject := range subjects {
		if subject.AccountGeneration != account.AccountGeneration || subject.OwnerCredentialHash != sha256.Sum256([]byte(owner.CredentialsCipher)) {
			t.Fatal("readiness reset reused old owner provenance", subject)
		}
	}
}

func TestFirstSyncAcknowledgmentOnlyWakesNewUsableProof(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name     string
		evidence *appliedMemberConfig
		revision uint64
		wantWake bool
	}{
		{name: "no_pending_proof", revision: 7},
		{name: "stale_revision", revision: 6, evidence: &appliedMemberConfig{revision: 7, principals: []FirstSyncSubject{{ProxyUserID: 11}}}},
		{name: "duplicate_ack", revision: 7, evidence: &appliedMemberConfig{revision: 7, principals: []FirstSyncSubject{{ProxyUserID: 11}}, appliedAt: now}},
		{name: "expired_proof", revision: 7, evidence: &appliedMemberConfig{revision: 7, principals: []FirstSyncSubject{{ProxyUserID: 11}}, validUntil: map[int64]time.Time{11: now}}},
		{name: "new_proof", revision: 7, evidence: &appliedMemberConfig{revision: 7, principals: []FirstSyncSubject{{ProxyUserID: 11}}}, wantWake: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(nil, nil, nil, nil, nil)
			service.now = func() time.Time { return now }
			master := NewMaster(service, nil)
			peer := &agentPeer{nodeID: "node", revision: 7, firstSync: test.evidence}
			master.peers[peer.nodeID] = peer
			master.recordFirstSyncAck(peer, test.revision)
			select {
			case <-master.stateWake:
				if !test.wantWake {
					t.Fatal("redundant acknowledgment triggered a fleet reconciliation")
				}
			default:
				if test.wantWake {
					t.Fatal("new applied membership did not wake proof persistence")
				}
			}
		})
	}
	if evidence := firstSyncCandidates(wire.Config{}, 7, now, map[int64]FirstSyncSubject{11: {ProxyUserID: 11}}); evidence != nil {
		t.Fatal("configuration with no eligible principal retained proof")
	}
}
