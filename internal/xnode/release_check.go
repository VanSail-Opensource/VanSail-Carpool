package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/upstreamqueue"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"github.com/VanSail-Opensource/X-Node/pkg/releases"
	"io"
	"net/http"
	"regexp"
	"time"
)

type ReleaseEvidence struct {
	Version        string `json:"version"`
	BinarySHA256   string `json:"binarySha256"`
	ManifestSHA256 string `json:"manifestSha256"`
	Platform       string `json:"platform"`
}

var stableRelease = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func (m *Master) AgentBuild(id string) (wire.AgentHello, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.peers[id] == nil {
		return wire.AgentHello{}, ErrUnavailable
	}
	return wire.Clone(m.live[id].hello), nil
}

// VerifyRelease compares the enrolled binary measurement with the same trusted
// signed release manifest used by the agent's updater.
func (s *Service) VerifyRelease(ctx context.Context, node string) (ReleaseEvidence, error) {
	hello, err := s.master.AgentBuild(node)
	if err != nil {
		return ReleaseEvidence{}, err
	}
	if !stableRelease.MatchString(hello.Version) || len(hello.BinarySHA256) != 64 || (hello.Platform != "linux-amd64" && hello.Platform != "linux-arm64") || hello.UpgradePending {
		return ReleaseEvidence{}, ErrUnavailable
	}
	supported := map[string]bool{}
	for _, name := range hello.Capabilities {
		supported[name] = true
	}
	for _, name := range []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "authorization_expiry", "measured_user_rates", "member_outbound_guards", "config_validation_v1"} {
		if !supported[name] {
			return ReleaseEvidence{}, ErrUnavailable
		}
	}
	return upstreamqueue.Do(ctx, s.validationQueue, func(ctx context.Context) (ReleaseEvidence, error) {
		base := "https://github.com/VanSail-Opensource/X-Node/releases/download/" + hello.Version + "/"
		manifest, err := releaseFile(ctx, base+"SHA256SUMS", 1<<20)
		if err != nil {
			return ReleaseEvidence{}, err
		}
		signature, err := releaseFile(ctx, base+"SHA256SUMS.minisig", 4096)
		if err != nil {
			return ReleaseEvidence{}, err
		}
		hashes, err := releases.Verify(manifest, signature, hello.Version)
		if err != nil {
			return ReleaseEvidence{}, ErrUnavailable
		}
		if hashes["x-node-"+hello.Platform] != hello.BinarySHA256 {
			return ReleaseEvidence{}, ErrUnavailable
		}
		digest := sha256.Sum256(manifest)
		return ReleaseEvidence{Version: hello.Version, BinarySHA256: hello.BinarySHA256, Platform: hello.Platform, ManifestSHA256: hex.EncodeToString(digest[:])}, nil
	})
}
func releaseFile(ctx context.Context, target string, max int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{Transport: publicTransport(), Timeout: 15 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 || !validHTTPS(r.URL.String()) {
			return ErrInvalid
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, ErrUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, max+1))
	if err != nil || int64(len(raw)) > max {
		return nil, ErrUnavailable
	}
	return raw, nil
}
func (m *Master) WaitApplied(ctx context.Context, nodeID string) error {
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		node, err := m.service.repo.XNodeNode(ctx, nodeID)
		if err != nil {
			return err
		}
		global, err := m.service.Global(ctx)
		if err != nil {
			return err
		}
		m.mu.RLock()
		snapshot := m.live[nodeID]
		connected := m.peers[nodeID] != nil
		fresh := connected && freshSnapshot(snapshot, m.service.now(), global.Telemetry.ReportSeconds)
		firewallConfirmed := snapshot.firewallPending == ""
		report := snapshot.report
		m.mu.RUnlock()
		if fresh && firewallConfirmed && node.AppliedRevision >= node.DesiredRevision && report.ConfigRevision >= node.DesiredRevision && report.Ready && report.Synchronized {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}
