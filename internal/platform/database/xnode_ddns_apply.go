package database

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Only address fields change; listener settings, credentials, member choices
// and accounting remain untouched. The same transaction invalidates caches.
func (s *Store) retryAttempt_ApplyXNodeDDNSChange(ctx context.Context, state xnode.DDNSChangeState, in xnode.DDNSChangeInput, now time.Time) error {
	if in.Change != state.Change || len(in.Binding.IPv4HostIDs) > 0 && state.Change.IPv4 == "" || len(in.Binding.IPv6HostIDs) > 0 && state.Change.IPv6 == "" {
		return xnode.ErrInvalid
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	node, err := scanXNodeNode(tx.QueryRowContext(ctx, xnodeNodeSelect+` WHERE id=$1 FOR UPDATE`, state.NodeID))
	if err != nil {
		return err
	}
	if node.RemovingAt != nil || node.RetiredAt != nil || node.AgentID != state.AgentID || node.AgentID != in.Node.AgentID || node.SettingsCipher != in.Node.SettingsCipher {
		return xnode.ErrConflict
	}
	var version int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM xnode_compile_revision WHERE singleton=1 FOR UPDATE`).Scan(&version); err != nil {
		return err
	}
	if version != in.Version {
		return xnode.ErrConflict
	}
	current, err := scanDDNSChange(tx.QueryRowContext(ctx, ddnsChangeSelect+` WHERE node_id=$1 AND job_id=$2`, state.NodeID, state.Change.JobID))
	if err != nil {
		return err
	}
	if current.OperationID != state.OperationID || current.Phase != "staging" || current.Change != state.Change {
		return xnode.ErrConflict
	}
	previous := slices.Clone(current.PreviousIPs)
	for _, family := range []struct {
		name    string
		hosts   []string
		address string
	}{{"ipv4", in.Binding.IPv4HostIDs, state.Change.IPv4}, {"ipv6", in.Binding.IPv6HostIDs, state.Change.IPv6}} {
		for _, host := range family.hosts {
			var old string
			if err = tx.QueryRowContext(ctx, `SELECT e.address FROM xnode_endpoints e JOIN xnode_ddns_hosts h ON h.host_id=e.id WHERE h.node_id=$1 AND h.job_id=$2 AND h.family=$3 AND h.host_id=$4`, state.NodeID, state.Change.JobID, family.name, host).Scan(&old); err != nil {
				return err
			}
			if ip, e := xnode.FirewallPublicIP(old); e == nil {
				previous = append(previous, ip)
			}
			if _, err = tx.ExecContext(ctx, `UPDATE xnode_endpoints SET address=$1,updated_at=$2 WHERE id=$3 AND node_id=$4 AND address<>$5`, family.address, stamp(now), host, state.NodeID, family.address); err != nil {
				return err
			}
		}
	}
	slices.Sort(previous)
	previous = slices.Compact(previous)
	if len(previous) > 4096 {
		return xnode.ErrUnavailable
	}
	raw, err := json.Marshal(previous)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1`); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE xnode_ddns_changes SET phase='routes',previous_ips=$1 WHERE node_id=$2 AND job_id=$3 AND operation_id=$4`, string(raw), state.NodeID, state.Change.JobID, state.OperationID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AdvanceXNodeDDNSChange(ctx context.Context, state xnode.DDNSChangeState, phase string, previous []string) error {
	if phase != "settled" && phase != "obsolete" {
		return xnode.ErrInvalid
	}
	raw, err := json.Marshal(previous)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE xnode_ddns_changes SET phase=$1,previous_ips=$2 WHERE node_id=$3 AND job_id=$4 AND operation_id=$5 AND phase=$6`, phase, string(raw), state.NodeID, state.Change.JobID, state.OperationID, state.Phase)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return xnode.ErrConflict
	}
	return err
}
