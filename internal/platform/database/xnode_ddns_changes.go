package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

const ddnsChangeSelect = `SELECT node_id,job_id,agent_id,event_id,generation,sequence,ipv4,ipv6,observed_at,previous_ips,operation_id,phase FROM xnode_ddns_changes`

func scanDDNSChange(row rowScanner) (xnode.DDNSChangeState, error) {
	var state xnode.DDNSChangeState
	var sequence, observed, previous string
	c := &state.Change
	err := row.Scan(&state.NodeID, &c.JobID, &state.AgentID, &c.ID, &c.Generation, &sequence, &c.IPv4, &c.IPv6, &observed, &previous, &state.OperationID, &state.Phase)
	if errors.Is(err, sql.ErrNoRows) {
		return state, xnode.ErrNotFound
	}
	if err != nil {
		return state, err
	}
	c.Sequence, err = strconv.ParseUint(sequence, 10, 64)
	if err != nil {
		return state, err
	}
	c.ObservedAt, err = time.Parse(time.RFC3339Nano, observed)
	if err != nil {
		return state, err
	}
	err = json.Unmarshal([]byte(previous), &state.PreviousIPs)
	return state, err
}

func (s *Store) XNodeDDNSChanges(ctx context.Context) ([]xnode.DDNSChangeState, error) {
	rows, err := s.db.QueryContext(ctx, ddnsChangeSelect+` ORDER BY node_id,job_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := []xnode.DDNSChangeState{}
	for rows.Next() {
		state, err := scanDDNSChange(rows)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

// The node lock fences configuration saves and authenticated re-enrollment.
// The revision lock fences global provider changes and host-binding changes.
func (s *Store) retryAttempt_AcceptXNodeDDNSChange(ctx context.Context, in xnode.DDNSChangeInput, now time.Time) (bool, error) {
	if in.Change.Validate() != nil || in.Operation.Kind != xnode.DDNSOperationKind || in.Operation.ActorUserID != in.Binding.ConfiguredBy {
		return false, xnode.ErrInvalid
	}
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	node, err := scanXNodeNode(tx.QueryRowContext(ctx, xnodeNodeSelect+` WHERE id=? /*pg:FOR UPDATE*/`, in.Node.ID))
	if err != nil {
		return false, err
	}
	if node.AgentID != in.Node.AgentID || node.SettingsCipher != in.Node.SettingsCipher {
		return false, xnode.ErrConflict
	}
	var version int64
	if err = tx.QueryRowContext(ctx, `SELECT revision FROM xnode_compile_revision WHERE singleton=1 /*pg:FOR UPDATE*/`).Scan(&version); err != nil {
		return false, err
	}
	if version != in.Version {
		return false, xnode.ErrConflict
	}
	var actor string
	err = tx.QueryRowContext(ctx, `SELECT configured_by FROM xnode_ddns_jobs WHERE node_id=? AND job_id=?`, node.ID, in.Change.JobID).Scan(&actor)
	if err != nil {
		return false, err
	}
	if actor != in.Binding.ConfiguredBy {
		return false, xnode.ErrConflict
	}
	prior, err := scanDDNSChange(tx.QueryRowContext(ctx, ddnsChangeSelect+` WHERE node_id=? AND job_id=?`, node.ID, in.Change.JobID))
	if err != nil && !errors.Is(err, xnode.ErrNotFound) {
		return false, err
	}
	if err == nil && prior.AgentID == node.AgentID && in.Change.Sequence <= prior.Change.Sequence {
		return false, tx.Commit() // duplicate or out-of-order delivery has no effects
	}
	ips := slices.Clone(prior.PreviousIPs)
	for _, address := range in.PreviousIPs {
		if ip, e := xnode.FirewallPublicIP(address); e == nil {
			ips = append(ips, ip)
		}
	}
	for _, ip := range []string{prior.Change.IPv4, prior.Change.IPv6} {
		if ip != "" {
			ips = append(ips, ip)
		}
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.address FROM xnode_endpoints e JOIN xnode_ddns_hosts h ON e.id=h.host_id WHERE h.node_id=? AND h.job_id=?`, node.ID, in.Change.JobID)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var address string
		if err = rows.Scan(&address); err != nil {
			rows.Close()
			return false, err
		}
		if ip, e := xnode.FirewallPublicIP(address); e == nil {
			ips = append(ips, ip)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	slices.Sort(ips)
	ips = slices.Compact(ips)
	if len(ips) > 4096 {
		return false, xnode.ErrUnavailable
	}
	previous, err := json.Marshal(ips)
	if err != nil {
		return false, err
	}
	op, _, err := createProviderOperationTx(ctx, tx, in.Operation, now)
	if err != nil {
		return false, err
	}
	c := in.Change
	_, err = tx.ExecContext(ctx, `INSERT INTO xnode_ddns_changes(node_id,job_id,agent_id,event_id,generation,sequence,ipv4,ipv6,observed_at,previous_ips,operation_id,phase)
 VALUES(?,?,?,?,?,?,?,?,?,?,?,'staging') ON CONFLICT(node_id,job_id) DO UPDATE SET agent_id=excluded.agent_id,event_id=excluded.event_id,generation=excluded.generation,sequence=excluded.sequence,ipv4=excluded.ipv4,ipv6=excluded.ipv6,observed_at=excluded.observed_at,previous_ips=excluded.previous_ips,operation_id=excluded.operation_id,phase='staging'`, node.ID, c.JobID, node.AgentID, c.ID, c.Generation, strconv.FormatUint(c.Sequence, 10), c.IPv4, c.IPv6, stamp(c.ObservedAt), string(previous), op.Receipt.ID)
	if err != nil {
		return false, err
	}
	return true, tx.Commit()
}
