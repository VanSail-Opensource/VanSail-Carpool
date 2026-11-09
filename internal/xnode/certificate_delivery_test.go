package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type certificateReplayRepository struct {
	Repository
	node Node
}

func (r certificateReplayRepository) XNodeNode(context.Context, string) (Node, error) {
	return r.node, nil
}

func TestCertificateReplayUsesOriginalAgentAndRevisionAfterDesiredStateChanges(t *testing.T) {
	request := CommandRequest{Name: "certificates.run", ProfileID: "primary", Action: "renew", ExpectedRevision: 5}
	peer := &agentPeer{nodeID: "node", agentID: "original"}
	master := &Master{peers: map[string]*agentPeer{"node": peer}, live: map[string]liveSnapshot{"node": {hello: wire.AgentHello{Capabilities: []string{wire.CertificateLifecycleCapability}}}}}
	service := &Service{master: master, repo: certificateReplayRepository{node: Node{ID: "node", AgentID: "original", DesiredRevision: 8}}}
	intent := queuedNodeCommand{CommandRequest: request, AgentID: "original"}
	args, err := service.operationArguments(context.Background(), "node", providerops.Operation{Attempts: 1}, intent)
	if err != nil || args["expected_revision"] != uint64(5) {
		t.Fatal("desired changes blocked original durable replay", args, err)
	}
	peer.agentID = "replacement"
	if _, err = service.operationArguments(context.Background(), "node", providerops.Operation{Attempts: 1}, intent); !errors.Is(err, errCertificateEvidence) {
		t.Fatal("replacement peer could repeat uncertain renewal", err)
	}
	service.repo = certificateReplayRepository{node: Node{ID: "node", AgentID: "replacement", DesiredRevision: 8}}
	if _, err = service.operationArguments(context.Background(), "node", providerops.Operation{Attempts: 1}, intent); !errors.Is(err, errCertificateEvidence) {
		t.Fatal("reenrollment erased original-agent fence", err)
	}
}

func TestQueuedCertificateAgentBindingIsEncryptedAndNotBrowserInput(t *testing.T) {
	vault, err := secret.NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{vault: vault}
	request := CommandRequest{Name: "certificates.run", ProfileID: "primary", Action: "issue", ExpectedRevision: 5}
	raw, _ := json.Marshal(request)
	sealed, err := service.sealCommandIntent("actor", "key", Node{AgentID: "original"}, request, raw)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := vault.Decrypt("xnode.command.actor.key", sealed)
	if err != nil {
		t.Fatal(err)
	}
	var intent queuedNodeCommand
	if json.Unmarshal([]byte(opened), &intent) != nil || intent.AgentID != "original" || intent.CommandRequest != request {
		t.Fatal("queued intent lost original agent binding")
	}
	if _, err = service.sealCommandIntent("actor", "key", Node{}, request, raw); !errors.Is(err, ErrUnavailable) {
		t.Fatal("unbound node accepted lifecycle intent", err)
	}
}
