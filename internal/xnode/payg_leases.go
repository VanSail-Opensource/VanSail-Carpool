package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"slices"
	"strconv"
	"strings"
	"time"
)

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
	r, ok := m.service.paygRepository()
	if !ok {
		return config, nil
	}
	accounts, err := r.XNodePAYGAccounts(ctx, "", peer.nodeID)
	if err != nil {
		return config, err
	}
	if len(accounts) == 0 {
		return config, nil
	}
	byID := map[int]PAYGAccount{}
	for _, a := range accounts {
		byID[int(a.ProxyUserID)] = a
	}
	m.mu.RLock()
	live := m.live[peer.nodeID]
	m.mu.RUnlock()
	peer.mu.Lock()
	ready := peer.paygReady
	queued := peer.paygRecoveryQueued && m.service.now().Sub(peer.paygRecoveryAt) < 30*time.Second
	peer.mu.Unlock()
	healthy := paygCapabilities(caps) && freshSnapshot(live, m.service.now(), config.Telemetry.ReportSeconds) && live.report.Ready && live.report.Synchronized
	if healthy && !ready && !queued {
		raw, _ := json.Marshal(map[string]string{"nodeId": peer.nodeID, "bootId": peer.bootID})
		if err = r.EnqueueOutbox(ctx, PAYGRecoveryJob, string(raw), m.service.now()); err != nil {
			return config, err
		}
		peer.mu.Lock()
		peer.paygRecoveryQueued = true
		peer.paygRecoveryAt = m.service.now()
		peer.mu.Unlock()
	}
	until := time.Unix(m.service.now().Unix()/20*20+60, 0).UTC()
	users := make([]wire.User, 0, len(config.Users))
	hash := sha256.New()
	_, _ = hash.Write([]byte(strconv.FormatInt(until.Unix(), 10)))
	for _, u := range config.Users {
		a, payg := byID[u.ID]
		if !payg {
			users = append(users, u)
			continue
		}
		if a.Disabled || !healthy || !ready || !a.Funded(m.service.now()) {
			continue
		}
		validUntil := until
		if a.BalanceMinor < a.UnitPriceMinor && a.HasCredit(m.service.now()) && a.CreditStartedAt.Add(PAYGWindow).Before(validUntil) {
			validUntil = a.CreditStartedAt.Add(PAYGWindow)
		}
		u.ValidUntil = &validUntil
		_, _ = hash.Write([]byte("/" + strconv.Itoa(u.ID) + "@" + validUntil.Format(time.RFC3339Nano)))
		users = append(users, u)
	}
	config.Users = users
	key := hex.EncodeToString(hash.Sum(nil))
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
	if _, err := m.Flush(ctx, input.NodeID, 0, false); err != nil {
		return err
	}
	m.mu.RLock()
	same := m.peers[input.NodeID] == peer
	m.mu.RUnlock()
	if !same {
		return nil
	}
	peer.mu.Lock()
	peer.paygReady = true
	peer.paygRecoveryQueued = false
	peer.mu.Unlock()
	m.requestSync(peer)
	return nil
}

func isPAYGPeriod(id string) bool { return strings.HasPrefix(id, "payg-") }
