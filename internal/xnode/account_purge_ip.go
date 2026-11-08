package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// AccountPurgeIPControl retains the encrypted target until every captured
// positive subject has an acknowledged removal. It never represents a global IP.
type AccountPurgeIPControl struct {
	Block           connections.IPBlockRecord
	AgentID         string
	Principals      []int64
	MutationPending bool
}

type accountPurgeIPRepository interface {
	AccountPurgeIPControls(context.Context, model.AccountPurge, string) ([]AccountPurgeIPControl, bool, error)
	AdvanceAccountPurgeIPControl(context.Context, model.AccountPurge, string, string) (bool, error)
	RecordAccountPurgeIPClear(context.Context, model.AccountPurge, string, AccountPurgeIPControl) (bool, error)
}

func (s *Service) closeAccountPurgeIPControls(ctx context.Context, guard accountNativePurgeRepository, target model.AccountPurge, jobID string) error {
	repo, ok := s.repo.(accountPurgeIPRepository)
	if !ok {
		return ErrUnavailable
	}
	controls, active, err := repo.AccountPurgeIPControls(ctx, target, jobID)
	if err != nil || !active {
		return err
	}
	var pending error
	for _, control := range controls {
		allowed, e := repo.AdvanceAccountPurgeIPControl(ctx, target, jobID, control.Block.ID)
		if e == nil && !allowed {
			return nil
		}
		if e == nil {
			e = s.clearAccountPurgeIPControl(ctx, guard, target, jobID, control)
		}
		if e == nil {
			_, e = repo.RecordAccountPurgeIPClear(ctx, target, jobID, control)
		}
		if e != nil {
			pending = fmt.Errorf("%w; block %s: %w", pendingOrAccounting(pending), control.Block.ID, e)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	remaining, active, err := repo.AccountPurgeIPControls(ctx, target, jobID)
	if err != nil {
		return err
	}
	if !active {
		return nil
	}
	if len(remaining) > 0 {
		return fmt.Errorf("IP control scan pending: %w", pendingOrAccounting(pending))
	}
	return pending
}

func pendingOrAccounting(err error) error {
	if err == nil {
		return ErrAccessAccountingPending
	}
	return err
}

func (s *Service) clearAccountPurgeIPControl(ctx context.Context, guard accountNativePurgeRepository, target model.AccountPurge, jobID string, control AccountPurgeIPControl) error {
	if control.MutationPending || control.AgentID == "" || len(control.Principals) == 0 || s.master == nil || s.vault == nil {
		return &PolicyError{"ACCOUNT_PURGE_IP_EVIDENCE_PENDING", ErrAccessAccountingPending}
	}
	raw, err := s.vault.Decrypt("connection-ip-block:"+target.UserID+":"+control.Block.NodeUUID+":"+control.Block.IPDigest, control.Block.SealedIP)
	if err != nil {
		return &PolicyError{"ACCOUNT_PURGE_IP_TARGET_UNAVAILABLE", ErrAccessAccountingPending}
	}
	address, err := netip.ParseAddr(raw)
	if err != nil || address.IsUnspecified() || address.IsMulticast() || address.Zone() != "" {
		return ErrInvalid
	}
	for _, principal := range control.Principals {
		if err = s.accountPurgeStillAdmitted(ctx, guard, target); err != nil {
			return err
		}
		peer := s.master.accountPurgeIPPeer(control, principal)
		if peer == nil {
			return &PolicyError{"ACCOUNT_PURGE_IP_EVIDENCE_PENDING", ErrAccessAccountingPending}
		}
		hash := sha256.Sum256([]byte(fmt.Sprintf("purge-ip:%s:%d:%s:%s:%d", jobID, target.Generation, control.Block.ID, control.AgentID, principal)))
		result, e := s.master.commandPeer(ctx, peer, hex.EncodeToString(hash[:]), "ips.control", wire.IPControl{UserID: int(principal), SourceIP: address.Unmap().String(), Unblock: true})
		if e != nil {
			return e
		}
		var reply struct {
			UserID int        `json:"user_id"`
			IP     string     `json:"source_ip"`
			Until  *time.Time `json:"blocked_until"`
		}
		if json.Unmarshal(result, &reply) != nil || int64(reply.UserID) != principal || reply.IP != address.Unmap().String() || reply.Until == nil || !reply.Until.IsZero() {
			return ErrInvalid
		}
	}
	return nil
}

func (m *Master) accountPurgeIPPeer(control AccountPurgeIPControl, principal int64) *agentPeer {
	if principal <= 0 || principal >= 1<<53 {
		return nil
	}
	m.mu.RLock()
	peer := m.peers[control.Block.NodeUUID]
	live := m.live[control.Block.NodeUUID]
	m.mu.RUnlock()
	if peer == nil || peer.agentID != control.AgentID || live.report.ConfigRevision == 0 || !live.report.Ready || !live.report.Synchronized {
		return nil
	}
	peer.syncMu.Lock()
	defer peer.syncMu.Unlock()
	peer.mu.Lock()
	revision, digest := peer.revision, peer.digest
	peer.mu.Unlock()
	if peer.wireState == nil || digest == "" || digest != peer.wireState.digest || live.report.ConfigRevision != revision {
		return nil
	}
	for _, user := range peer.wireState.config.Users {
		if int64(user.ID) == principal && len(user.InboundTags) > 0 && len(user.Credentials) > 0 {
			return peer
		}
	}
	return nil
}
