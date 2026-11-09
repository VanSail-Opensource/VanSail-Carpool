package xnode

import (
	"context"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestCertificateProgressDoesNotConsumeCommandOrTrafficCapacity(t *testing.T) {
	stamp := time.Now().UTC()
	peer := &agentPeer{nodeID: "node", pending: map[string]chan wire.Message{"job": make(chan wire.Message, 1)},
		certificateCommands: map[string]wire.CertificateRunArgs{"job": {ProfileID: "primary", Action: "issue", ExpectedRevision: 5}}}
	master := &Master{service: &Service{now: func() time.Time { return stamp }}, peers: map[string]*agentPeer{"node": peer}, live: map[string]liveSnapshot{}}
	for _, phase := range []string{"issuing", "applying"} {
		if err := master.receive(context.Background(), peer, wire.New("command.progress", "job", 0, wire.CertificateRunProgress{ProfileID: "primary", Action: "issue", Phase: phase})); err != nil {
			t.Fatal(err)
		}
	}
	if len(peer.pending["job"]) != 0 || master.live["node"].certificateProgress.Phase != "applying" {
		t.Fatal("advisory progress displaced terminal command capacity")
	}
	if err := master.receiveCertificateProgress(peer, wire.New("command.progress", "job", 0, wire.CertificateRunProgress{ProfileID: "other", Action: "issue", Phase: "issuing"})); err == nil {
		t.Fatal("wrong profile progress accepted")
	}
	master.clearCertificateProgress(peer, "different-job")
	if master.live["node"].certificateProgress == nil {
		t.Fatal("unrelated command cleared certificate progress")
	}
	master.clearCertificateProgress(peer, "job")
	if master.live["node"].certificateProgress != nil {
		t.Fatal("terminal progress retained")
	}
	if err := master.receiveCertificateProgress(peer, wire.New("command.progress", "untracked", 0, map[string]string{"other": "utility"})); err != nil {
		t.Fatal("other utility progress incorrectly rejected", err)
	}
}
