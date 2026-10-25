package database

import (
	"context"
	"database/sql"
	"slices"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Store) XNodeDDNSBindings(ctx context.Context, node string) (map[string]xnode.DDNSHostBinding, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT j.job_id,j.configured_by,h.family,h.host_id FROM xnode_ddns_jobs j LEFT JOIN xnode_ddns_hosts h ON h.node_id=j.node_id AND h.job_id=j.job_id WHERE j.node_id=? ORDER BY j.job_id,h.family,h.host_id`, node)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]xnode.DDNSHostBinding{}
	for rows.Next() {
		var job, owner string
		var family, host sql.NullString
		if err = rows.Scan(&job, &owner, &family, &host); err != nil {
			return nil, err
		}
		binding := result[job]
		binding.ConfiguredBy = owner
		if family.String == "ipv4" {
			binding.IPv4HostIDs = append(binding.IPv4HostIDs, host.String)
		}
		if family.String == "ipv6" {
			binding.IPv6HostIDs = append(binding.IPv6HostIDs, host.String)
		}
		result[job] = binding
	}
	return result, rows.Err()
}

// Discovery settings and host references commit together. Host ownership and
// single-job binding are also authoritative FK/unique constraints.
func (s *Store) retryAttempt_SaveXNodeNodeDDNS(ctx context.Context, node xnode.Node, bindings map[string]xnode.DDNSHostBinding) error {
	if !IsPostgres(s.db) {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, saveXNodeNodeSQL, xnodeNodeArgs(node)...); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM xnode_ddns_jobs WHERE node_id=?`, node.ID); err != nil {
		return err
	}
	keys := make([]string, 0, len(bindings))
	for job := range bindings {
		keys = append(keys, job)
	}
	slices.Sort(keys)
	for _, job := range keys {
		binding := bindings[job]
		if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_ddns_jobs(node_id,job_id,configured_by) VALUES(?,?,?)`, node.ID, job, binding.ConfiguredBy); err != nil {
			return err
		}
		for _, family := range []struct {
			name  string
			hosts []string
		}{{"ipv4", binding.IPv4HostIDs}, {"ipv6", binding.IPv6HostIDs}} {
			for _, host := range family.hosts {
				if _, err = tx.ExecContext(ctx, `INSERT INTO xnode_ddns_hosts(node_id,job_id,family,host_id) VALUES(?,?,?,?)`, node.ID, job, family.name, host); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}
