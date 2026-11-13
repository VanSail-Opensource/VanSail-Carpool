package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const invalidStoredDDNSCode = "XNODE_DDNS_JOB_INVALID"
const maxDDNSDiagnosticJobs = 8

type ddnsCompileDiagnostic struct {
	ids   []string
	count int
}

// Persisted jobs predate some execution checks. An unusable utility job must
// not prevent native or authorization publication. This projection never edits
// encrypted settings, host bindings or the last accepted address observations;
// ordinary SaveNode and incoming agent configurations remain strictly validated.
func compiledDDNSJobs(node string, settings NodeSettings, providers map[string]wire.Provider) ([]wire.DDNSJob, ddnsCompileDiagnostic) {
	jobs := ddnsJobsForNode(node, settings, providers)
	valid := jobs[:0]
	var diagnostic ddnsCompileDiagnostic
	for _, job := range jobs {
		if job.Validate() != nil {
			diagnostic.count++
			if len(diagnostic.ids) < maxDDNSDiagnosticJobs {
				diagnostic.ids = append(diagnostic.ids, ddnsDiagnosticID(job.ID))
			}
			continue
		}
		valid = append(valid, job)
	}
	clear(jobs[len(valid):])
	return valid, diagnostic
}

func (s *Service) validateStoredDDNSJob(ctx context.Context, node, id string) error {
	current, err := s.repo.XNodeNode(ctx, node)
	if err != nil {
		return err
	}
	settings, err := s.openNodeSettings(current)
	if err != nil {
		return err
	}
	for _, job := range settings.DDNS {
		if job.ID == id {
			if job.Validate() != nil {
				return ErrInvalid
			}
			return nil
		}
	}
	return ErrInvalid
}

// Only an epoch-accepted candidate for the current peer may publish live
// diagnostics. Manual/preflight compilations and obsolete peers have no effect.
// One warning per node per minute appears in the bounded admin log view. Only
// identifiers and a stable code are exposed, never execution inputs or errors.
func (m *Master) publishDDNSDiagnostic(peer *agentPeer, diagnostic ddnsCompileDiagnostic) {
	now := m.service.now().UTC()
	m.mu.Lock()
	node := peer.nodeID
	if m.peers[node] != peer {
		m.mu.Unlock()
		return
	}
	snapshot := m.live[node]
	if diagnostic.count == 0 {
		if snapshot.ddnsWarning != nil {
			snapshot.ddnsWarning = nil
			m.live[node] = snapshot
		}
		m.mu.Unlock()
		return
	}
	entry, _ := json.Marshal(struct {
		Time   time.Time `json:"time"`
		Source string    `json:"source"`
		Text   string    `json:"text"`
	}{now, "master", fmt.Sprintf("%s: %d DDNS job(s) unavailable (%s); correct node DDNS settings", invalidStoredDDNSCode, diagnostic.count, strings.Join(diagnostic.ids, ", "))})
	snapshot.ddnsWarning = entry
	if !snapshot.ddnsWarningAt.IsZero() && now.Before(snapshot.ddnsWarningAt.Add(time.Minute)) {
		m.live[node] = snapshot
		m.mu.Unlock()
		return
	}
	snapshot.ddnsWarningAt = now
	snapshot.logs = append(snapshot.logs, entry)
	if len(snapshot.logs) > 500 {
		dropped := uint64(len(snapshot.logs) - 500)
		snapshot.logsDropped += min(dropped, ^uint64(0)-snapshot.logsDropped)
		snapshot.logs = snapshot.logs[len(snapshot.logs)-500:]
	}
	m.live[node] = snapshot
	m.mu.Unlock()
	if m.logger != nil {
		m.logger.Warn("stored DDNS jobs unavailable", "node_id", ddnsDiagnosticID(node), "job_ids", diagnostic.ids, "invalid_jobs", diagnostic.count, "error_code", invalidStoredDDNSCode)
	}
}

func ddnsDiagnosticID(id string) string {
	if id == "" || len(id) > 128 {
		return "<invalid-id>"
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-') {
			return "<invalid-id>"
		}
	}
	return id
}
