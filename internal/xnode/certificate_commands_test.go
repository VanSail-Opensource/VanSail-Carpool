package xnode

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestCertificateCommandsRejectUnfencedOrCrossCommandFields(t *testing.T) {
	valid := CommandRequest{Name: "certificates.run", ProfileID: "primary", Action: "issue", ExpectedRevision: 5}
	if !validNodeCommand(valid) || commandDeadline(valid.Name) != 5*time.Minute {
		t.Fatal("valid bounded lifecycle command rejected")
	}
	for _, mutate := range []func(*CommandRequest){
		func(r *CommandRequest) { r.ProfileID = "../secret" },
		func(r *CommandRequest) { r.Action = "retry" },
		func(r *CommandRequest) { r.ExpectedRevision = 0 },
		func(r *CommandRequest) { r.ExpectedRevision = 1 << 53 },
		func(r *CommandRequest) { r.Force = true },
		func(r *CommandRequest) { r.Name = "runtime.reload" },
		func(r *CommandRequest) { r.JobID = "unrelated" },
	} {
		request := valid
		mutate(&request)
		if validNodeCommand(request) {
			t.Fatal("invalid command accepted", request)
		}
	}
}

func TestCertificateOutcomeFactsRetainIntentAndExcludeSecrets(t *testing.T) {
	request := CommandRequest{Name: "certificates.run", ProfileID: "primary", Action: "renew", ExpectedRevision: 42}
	for _, raw := range []string{"", `{"profile_id":"other","action":"renew","key":"private"}`, `invalid`,
		`{"profile_id":"primary","action":"renew","fingerprint":"` + strings.Repeat("a", 64) + `","applied":true,"ready":false,"key":"private"}`} {
		encoded, err := certificateOperationResult(request, "node", json.RawMessage(raw))
		if err != nil {
			t.Fatal(err)
		}
		var facts map[string]any
		if json.Unmarshal([]byte(encoded), &facts) != nil || facts["action"] != "renew" || facts["expected_revision"] != float64(42) || facts["profile_id"] != "primary" {
			t.Fatal("uncertain result erased original intent", encoded)
		}
		if strings.Contains(encoded, "private") || strings.Contains(encoded, `"key"`) {
			t.Fatal("private agent fields entered a durable result")
		}
	}
	for _, fingerprint := range []string{"", strings.Repeat("g", 64), strings.Repeat("a", 63)} {
		raw, _ := json.Marshal(wire.CertificateRunResult{ProfileID: "primary", Action: "renew", Applied: true, Ready: true, Fingerprint: fingerprint})
		if _, err := validateCertificateResult(request, raw); err == nil {
			t.Fatal("unverified active result accepted", fingerprint)
		}
	}
}

func TestCertificateWaitKeepsFailureEvidenceAndIgnoresProgress(t *testing.T) {
	for _, status := range []string{"failed", "uncertain"} {
		pending := make(chan wire.Message, 3)
		pending <- wire.New("command.accepted", "job", 0, map[string]string{"command": "certificates.run"})
		pending <- wire.New("command.progress", "job", 0, wire.CertificateRunProgress{ProfileID: "primary", Action: "renew", Phase: "applying"})
		pending <- wire.New("command.result", "job", 0, map[string]any{"id": "job", "command": "certificates.run", "status": status, "data": wire.CertificateRunResult{ProfileID: "primary", Action: "renew", Applied: true}})
		raw, err := waitCommand(context.Background(), &agentPeer{ctx: context.Background()}, pending, "job", "certificates.run", nil)
		var outcome *CommandError
		if !errors.As(err, &outcome) || !strings.Contains(string(raw), `"applied":true`) {
			t.Fatal("partial activation evidence was lost", string(raw), err)
		}
	}
}
