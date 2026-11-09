package xnode

import (
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// CertificateCommandProgress is one bounded, memory-only advisory snapshot.
// Durable operation receipts remain authoritative for completion and recovery.
type CertificateCommandProgress struct {
	CommandID  string    `json:"command_id"`
	ProfileID  string    `json:"profile_id"`
	Action     string    `json:"action"`
	Phase      string    `json:"phase"`
	ReceivedAt time.Time `json:"received_at"`
}

func (m *Master) receiveCertificateProgress(peer *agentPeer, message wire.Message) error {
	peer.mu.Lock()
	expected, tracked := peer.certificateCommands[message.ID]
	peer.mu.Unlock()
	if !tracked {
		return nil // Other utilities can publish their own advisory payloads.
	}
	if len(message.Data) > 1024 {
		return ErrInvalid
	}
	var progress wire.CertificateRunProgress
	if wire.Decode(message.Data, &progress) != nil || progress.ProfileID != expected.ProfileID || progress.Action != expected.Action {
		return ErrInvalid
	}
	switch progress.Phase {
	case "issuing", "renewing", "applying":
	default:
		return ErrInvalid
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.peers[peer.nodeID] != peer {
		return nil
	}
	snapshot := m.live[peer.nodeID]
	snapshot.certificateProgress = &CertificateCommandProgress{CommandID: message.ID, ProfileID: progress.ProfileID, Action: progress.Action, Phase: progress.Phase, ReceivedAt: m.service.now()}
	m.live[peer.nodeID] = snapshot
	return nil
}

func (m *Master) clearCertificateProgress(peer *agentPeer, command string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := m.live[peer.nodeID]
	if m.peers[peer.nodeID] == peer && snapshot.certificateProgress != nil && snapshot.certificateProgress.CommandID == command {
		snapshot.certificateProgress = nil
		m.live[peer.nodeID] = snapshot
	}
}
