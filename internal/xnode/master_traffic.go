package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// trafficRetry is guarded by peer.mu. Retain one fingerprint, never a payload.
// Suppression never ACKs and never stalls the serial command/telemetry reader.
type trafficRetry struct {
	id     string
	digest [sha256.Size]byte
	delay  time.Duration
	after  time.Time
}

func (r trafficRetry) blocked(now time.Time, id string, digest [sha256.Size]byte) bool {
	return r.id == id && r.digest == digest && now.Before(r.after)
}

func (r *trafficRetry) reject(now time.Time, id string, digest [sha256.Size]byte) {
	delay := time.Second
	if r.id == id && r.digest == digest {
		delay = min(30*time.Second, max(delay, r.delay*2))
	}
	*r = trafficRetry{id: id, digest: digest, delay: delay, after: now.Add(delay)}
}

func (m *Master) receiveTraffic(ctx context.Context, peer *agentPeer, message wire.Message) error {
	now := m.service.now()
	fingerprint := sha256.Sum256(message.Data)
	peer.mu.Lock()
	blocked := peer.trafficRetry.blocked(now, message.ID, fingerprint)
	peer.mu.Unlock()
	if blocked {
		return nil
	}
	err := m.commitTraffic(ctx, peer, message)
	peer.mu.Lock()
	if err != nil {
		peer.trafficRetry.reject(m.service.now(), message.ID, fingerprint)
		peer.paygReady = false
		// Keep an existing durable recovery coalesced across journal retries.
		// overlayPAYG already bounds retries of interrupted recovery attempts.
	} else {
		peer.trafficRetry = trafficRetry{}
	}
	peer.mu.Unlock()
	return err
}

func (m *Master) commitTraffic(ctx context.Context, peer *agentPeer, message wire.Message) error {
	var report wire.TrafficReport
	if err := wire.Decode(message.Data, &report); err != nil {
		return ErrInvalid
	}
	raw, ok := strings.CutPrefix(report.ID, peer.agentID+":")
	if !ok || message.ID != report.ID {
		return ErrInvalid
	}
	sequence, err := strconv.ParseUint(raw, 10, 63)
	if err != nil {
		return ErrInvalid
	}
	normalized, err := json.Marshal(report)
	if err != nil {
		return ErrInvalid
	}
	digest := sha256.Sum256(normalized)
	commitStarted := time.Now()
	if err = m.service.repo.RecordXNodeTraffic(ctx, peer.nodeID, peer.agentID, sequence, hex.EncodeToString(digest[:]), report); err != nil {
		return trafficReportFailure{err: err, sequence: sequence, samples: len(report.Samples)}
	}
	committed := time.Now()
	m.trafficMetrics.commits.Add(1)
	m.trafficMetrics.commitNanos.Add(uint64(committed.Sub(commitStarted)))
	err = peer.send(ctx, wire.New("traffic.ack", "", 0, map[string]string{"id": report.ID}))
	m.trafficMetrics.ackNanos.Add(uint64(time.Since(committed)))
	if err == nil {
		m.trafficMetrics.acknowledged.Add(1)
	} else {
		m.trafficMetrics.ackFailures.Add(1)
	}
	return err
}
