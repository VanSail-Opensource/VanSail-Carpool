package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"time"

	"slices"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const maxPAYGAgentClockOffset = 2 * time.Minute
const paygRenewalInterval = 20 * time.Second
const paygLeaseLifetime = time.Minute

func paygLeaseUntil(now time.Time, live liveSnapshot) time.Time {
	base := paygAgentTime(now, live)
	return base.Truncate(paygRenewalInterval).Add(paygLeaseLifetime).UTC()
}

func paygAgentTime(at time.Time, live liveSnapshot) time.Time {
	offset := live.report.Time.Sub(live.receivedAt)
	if offset > 0 && offset <= maxPAYGAgentClockOffset {
		return at.Add(offset)
	}
	return at
}

func paygCapabilities(caps []string) bool {
	for _, capability := range []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards"} {
		if !slices.Contains(caps, capability) {
			return false
		}
	}
	return true
}

// Called under peer.syncMu. Only authorization is copied; the validated native
// document and its cached parser proof remain unchanged across lease renewals.
func (m *Master) overlayPAYG(ctx context.Context, peer *agentPeer, config wire.Config, caps []string) (wire.Config, error) {
	peer.paygHeld = false
	r, ok := m.service.paygRepository()
	if !ok {
		return config, nil
	}
	accounts, err := fundedPAYGAccounts(ctx, r, peer.nodeID)
	if err != nil {
		return config, err
	}
	if len(accounts) == 0 && !slices.ContainsFunc(config.Users, func(user wire.User) bool { return isPAYGPeriod(user.AccountingPeriod) }) {
		return config, nil
	}
	now := m.service.now()
	m.mu.RLock()
	live := m.live[peer.nodeID]
	current := m.peers[peer.nodeID] == peer
	m.mu.RUnlock()
	peer.mu.Lock()
	ready := peer.paygReady
	queued := peer.paygRecoveryQueued && now.Sub(peer.paygRecoveryAt) < 30*time.Second
	peer.mu.Unlock()
	usable := current && paygCapabilities(caps) && live.report.Ready && live.report.Synchronized && live.firewallPending == ""
	healthy := usable && freshSnapshot(live, now, config.Telemetry.ReportSeconds)
	hold := usable && !healthy && !live.receivedAt.IsZero()
	type fundingIndex struct {
		index   int
		blocked bool
	}
	byID := make(map[int]fundingIndex, len(accounts))
	admissions := map[string]bool{}
	for i := range accounts {
		a := &accounts[i]
		entry := fundingIndex{index: i, blocked: a.Disabled}
		if healthy && ready && !a.HasCredit(now) {
			if admission, ok := r.(interface {
				PricingPAYGAdmission(context.Context, string, string, time.Time) (bool, error)
			}); ok {
				safe, seen := admissions[a.GroupID]
				if !seen {
					var e error
					safe, e = admission.PricingPAYGAdmission(ctx, a.GroupID, a.NodeID, now)
					if e != nil {
						return config, e
					}
					admissions[a.GroupID] = safe
				}
				entry.blocked = entry.blocked || !safe
			}
		}
		byID[int(a.ProxyUserID)] = entry
	}
	if healthy && !ready && !queued {
		raw, _ := json.Marshal(map[string]string{"nodeId": peer.nodeID, "bootId": peer.bootID})
		if err = r.EnqueueOutbox(ctx, PAYGRecoveryJob, string(raw), now); err != nil {
			return config, err
		}
		peer.mu.Lock()
		peer.paygRecoveryQueued = true
		peer.paygRecoveryAt = now
		peer.mu.Unlock()
	}
	agentNow := paygAgentTime(now, live)
	until := paygLeaseUntil(now, live)
	users := make([]wire.User, 0, len(config.Users))
	hash := sha256.New()
	var keyMaterial [20]byte
	for _, u := range config.Users {
		entry, payg := byID[u.ID]
		if !payg && !isPAYGPeriod(u.AccountingPeriod) {
			users = append(users, u)
			continue
		}
		if !payg || !ready || entry.blocked {
			continue
		}
		a := &accounts[entry.index]
		if !a.Funded(now) {
			continue
		}
		validUntil := &until
		if !healthy {
			lease, exists := peer.paygPublished[u.ID]
			if !hold || !exists || lease.period != u.AccountingPeriod || !lease.until.After(agentNow) {
				continue
			}
			validUntil = &lease.until
		}
		if a.BalanceMinor < a.UnitPriceMinor && a.HasCredit(now) {
			creditUntil := paygAgentTime(a.CreditStartedAt.Add(PAYGWindow), live)
			if creditUntil.Before(*validUntil) {
				validUntil = &creditUntil
			}
		}
		peer.paygHeld = peer.paygHeld || !healthy
		u.ValidUntil = validUntil
		// This fingerprint is local to the prepared wire cache. Fixed-width
		// encoding retains exact expiry without formatting it for every user.
		binary.LittleEndian.PutUint64(keyMaterial[:8], uint64(u.ID))
		binary.LittleEndian.PutUint64(keyMaterial[8:16], uint64(validUntil.Unix()))
		binary.LittleEndian.PutUint32(keyMaterial[16:], uint32(validUntil.Nanosecond()))
		_, _ = hash.Write(keyMaterial[:])
		users = append(users, u)
	}
	config.Users = users
	var key [sha256.Size]byte
	hash.Sum(key[:0])
	if key != peer.paygOverlayKey {
		peer.wireState = nil
		peer.paygOverlayKey = key
	}
	return config, nil
}

func (m *Master) recoverPAYG(ctx context.Context, job model.OutboxJob) error {
	var input struct {
		NodeID string `json:"nodeId"`
		BootID string `json:"bootId"`
	}
	if json.Unmarshal([]byte(job.Payload), &input) != nil || input.NodeID == "" || input.BootID == "" {
		return ErrInvalid
	}
	m.mu.RLock()
	peer := m.peers[input.NodeID]
	m.mu.RUnlock()
	if peer == nil || peer.bootID != input.BootID {
		return nil
	}
	if err := m.Sync(ctx, input.NodeID); err != nil {
		return err
	}
	if err := m.WaitApplied(ctx, input.NodeID); err != nil {
		return err
	}
	peer.mu.Lock()
	failureVersion := peer.paygFailureVersion
	peer.mu.Unlock()
	if _, err := m.Flush(ctx, input.NodeID, 0, false); err != nil {
		return err
	}
	return m.completePAYGRecovery(peer, failureVersion)
}

func (m *Master) completePAYGRecovery(peer *agentPeer, failureVersion uint64) error {
	m.mu.RLock()
	same := m.peers[peer.nodeID] == peer
	m.mu.RUnlock()
	if !same {
		return nil
	}
	peer.mu.Lock()
	if peer.paygFailureVersion != failureVersion {
		peer.mu.Unlock()
		return ErrUnavailable
	}
	peer.paygReady = true
	peer.paygRecoveryQueued = false
	peer.mu.Unlock()
	m.requestSync(peer)
	return nil
}

func isPAYGPeriod(id string) bool { return strings.HasPrefix(id, "payg-") }
