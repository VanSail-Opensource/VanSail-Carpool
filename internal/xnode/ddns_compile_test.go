package xnode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestStoredDDNSProjectionPreservesSourceAndBoundsDiagnostics(t *testing.T) {
	service, repo, peer, master := compileCacheFixture(t)
	master.peers[peer.nodeID] = peer
	now := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }
	var logs bytes.Buffer
	master.logger = slog.New(slog.NewJSONHandler(&logs, nil))
	settings := NodeSettings{DDNSBindings: map[string]DDNSHostBinding{
		"valid": {IPv4HostIDs: []string{"known-host"}, ConfiguredBy: "admin"},
	}}
	for index := 0; index < 20; index++ {
		settings.DDNS = append(settings.DDNS, wire.DDNSJob{ID: fmt.Sprintf("invalid-%02d", index), Provider: "external", Zone: "example.com", Name: "@", IPv4URL: "https://private-lookup.example.com/?token=must-not-log"})
	}
	settings.DDNS = append(settings.DDNS, wire.DDNSJob{ID: "valid", Provider: "external", Zone: "example.com", Name: "@", IPv4: "8.8.8.8"})
	providers := map[string]wire.Provider{"external": {Type: "cloudflare", Credentials: map[string]string{"token": "provider-secret"}}}
	before := wire.Clone(settings)
	before.DDNSBindings["valid"] = DDNSHostBinding{IPv4HostIDs: []string{"known-host"}, ConfiguredBy: "admin"}
	projected, diagnostic := compiledDDNSJobs("node", settings, providers)
	if len(projected) != 1 || projected[0].ID != "valid" || projected[0].Generation == "" {
		t.Fatalf("valid sibling or binding generation was lost: %+v", projected)
	}
	if !reflect.DeepEqual(settings, before) {
		t.Fatal("compile projection changed saved jobs or bindings")
	}
	if len(master.live["node"].logs) != 0 || logs.Len() != 0 {
		t.Fatal("pure compile projection published live diagnostics")
	}
	for range 100 {
		master.publishDDNSDiagnostic(peer, diagnostic)
	}
	report, err := master.Logs(context.Background(), repo.node.ID)
	if err != nil || len(report.Entries) != 1 || bytes.Count(logs.Bytes(), []byte(invalidStoredDDNSCode)) != 1 {
		t.Fatal("invalid utility diagnostics were lost or repeated without a bound", report, err)
	}
	var entry struct{ Source, Text string }
	if json.Unmarshal(report.Entries[0], &entry) != nil || entry.Source != "master" || !strings.Contains(entry.Text, invalidStoredDDNSCode) || !strings.Contains(entry.Text, "20 DDNS") || !strings.Contains(entry.Text, "invalid-07") || strings.Contains(entry.Text, "invalid-08") {
		t.Fatal("admin log omitted stable diagnostic or exceeded identifier bound", string(report.Entries[0]))
	}
	for _, secret := range []string{"provider-secret", "must-not-log", "private-lookup"} {
		if bytes.Contains(logs.Bytes(), []byte(secret)) || bytes.Contains(report.Entries[0], []byte(secret)) {
			t.Fatal("utility diagnostic exposed execution inputs")
		}
	}
	master.mu.Lock()
	snapshot := master.live["node"]
	snapshot.logs = make([]json.RawMessage, 500)
	master.live["node"] = snapshot
	master.mu.Unlock()
	now = now.Add(time.Minute)
	master.publishDDNSDiagnostic(peer, diagnostic)
	if report, err = master.Logs(context.Background(), "node"); err != nil || len(report.Entries) != 500 || report.Dropped != "1" {
		t.Fatal("diagnostics exceeded the existing log ring", report, err)
	}
	_, corrected := compiledDDNSJobs("node", NodeSettings{DDNS: settings.DDNS[20:]}, providers)
	master.publishDDNSDiagnostic(peer, corrected)
	if master.live["node"].ddnsWarning != nil {
		t.Fatal("corrected settings retained a current utility failure")
	}
}

func TestObsoleteAndManualCompilationCannotPublishDDNSDiagnostics(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprintf("invalid-%t", invalid), func(t *testing.T) {
			service, repo, peer, master := compileCacheFixture(t)
			master.peers[peer.nodeID] = peer
			master.publishDDNSDiagnostic(peer, ddnsCompileDiagnostic{ids: []string{"current-job"}, count: 1})
			current := bytes.Clone(master.live["node"].ddnsWarning)
			settings := NodeSettings{}
			if invalid {
				settings.DDNS = []wire.DDNSJob{{ID: "obsolete-job", Provider: "external", Zone: "example.com", Name: "@"}}
			}
			var err error
			repo.node.SettingsCipher, err = service.sealNodeSettings("node", settings)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.Compile(context.Background(), "node"); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(master.live["node"].ddnsWarning, current) {
				t.Fatal("manual compilation mutated live diagnostics")
			}
			// A commit while this read set is compiling makes its final epoch
			// obsolete. Neither a warning nor its removal may escape that fence.
			repo.beforeRead = func() { repo.revision++ }
			if _, err := master.cachedConfiguration(context.Background(), peer, false); !errors.Is(err, ErrConflict) {
				t.Fatal("obsolete candidate was not rejected", err)
			}
			if !bytes.Equal(master.live["node"].ddnsWarning, current) {
				t.Fatal("epoch-rejected compilation changed the current diagnostic")
			}
			repo.beforeRead = nil
			if _, err := master.cachedConfiguration(context.Background(), peer, false); err != nil {
				t.Fatal(err)
			}
			published := master.live["node"].ddnsWarning
			if invalid && !bytes.Contains(published, []byte("obsolete-job")) || !invalid && published != nil {
				t.Fatal("accepted candidate did not publish its own diagnostic", string(published))
			}
		})
	}
}

func TestReplacedPeerCannotPublishDDNSDiagnostic(t *testing.T) {
	service, repo, peer, master := compileCacheFixture(t)
	replacement := &agentPeer{nodeID: peer.nodeID}
	master.peers[peer.nodeID] = replacement
	master.publishDDNSDiagnostic(replacement, ddnsCompileDiagnostic{ids: []string{"current-job"}, count: 1})
	current := bytes.Clone(master.live["node"].ddnsWarning)
	var err error
	repo.node.SettingsCipher, err = service.sealNodeSettings("node", NodeSettings{DDNS: []wire.DDNSJob{
		{ID: "obsolete-job", Provider: "external", Zone: "example.com", Name: "@"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := master.cachedConfiguration(context.Background(), peer, false); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(master.live["node"].ddnsWarning, current) {
		t.Fatal("obsolete peer changed replacement diagnostics")
	}
}

func TestStoredInvalidDDNSManualCommandIsRejectedBeforeFirstAttempt(t *testing.T) {
	service, repo, _, _ := compileCacheFixture(t)
	settings := NodeSettings{DDNS: []wire.DDNSJob{
		{ID: "invalid", Provider: "external", Zone: "example.com", Name: "@"},
		{ID: "valid", Provider: "external", Zone: "example.com", Name: "@", IPv4: "8.8.8.8"},
	}}
	var err error
	repo.node.SettingsCipher, err = service.sealNodeSettings("node", settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"invalid", "missing"} {
		request := queuedNodeCommand{CommandRequest: CommandRequest{Name: "ddns.run", JobID: id}}
		if _, err := service.operationArguments(context.Background(), "node", providerops.Operation{}, request); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid unattempted utility command accepted: %q, %v", id, err)
		}
		if args, err := service.operationArguments(context.Background(), "node", providerops.Operation{Attempts: 1}, request); err != nil || args["job_id"] != id {
			t.Fatal("current settings blocked retrieval of an earlier command receipt", args, err)
		}
	}
	request := queuedNodeCommand{CommandRequest: CommandRequest{Name: "ddns.run", JobID: "valid"}}
	if args, err := service.operationArguments(context.Background(), "node", providerops.Operation{}, request); err != nil || args["job_id"] != "valid" {
		t.Fatal("valid sibling command rejected", args, err)
	}
}
