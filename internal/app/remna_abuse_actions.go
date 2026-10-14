package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
)

func (a remnaAdapter) AbuseRevoke(ctx context.Context, remoteID string) error {
	id, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		_, callErr := client.RevokeSubscription(callCtx, id, false)
		return callErr
	})
}
func (a remnaAdapter) AbuseSetStatus(ctx context.Context, remoteID string, status remnawave.UserStatus) error {
	id, err := remnaUserID(remoteID)
	if err != nil {
		return err
	}
	return remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
		_, callErr := client.UpdateUser(callCtx, remnawave.UpdateUserRequest{ID: id, Status: &status})
		return callErr
	})
}
func (a remnaAdapter) StartAbuseIPBanScan(ctx context.Context, remoteID string) (string, error) {
	id, err := remnaUserID(remoteID)
	if err != nil {
		return "", err
	}
	return remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (string, error) {
		return client.RequestUserConnections(callCtx, id)
	})
}

func (a remnaAdapter) CompleteAbuseIPBan(ctx context.Context, scanID string, nodes []string, all bool, durationSeconds int) (bool, error) {
	scan, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) (remnawave.ConnectionScan, error) {
		return client.UserConnections(callCtx, scanID)
	})
	if err != nil {
		return false, err
	}
	if scan.Failed {
		return false, errors.New("abuse connection scan failed")
	}
	if !scan.Completed {
		return false, nil
	}
	wanted := map[string]bool{}
	for _, node := range nodes {
		wanted[node] = true
	}
	targets, err := a.abuseIPBanTargets(ctx, scan, wanted, all)
	if err != nil {
		return false, err
	}
	for nodeUUID, ips := range targets {
		for _, ip := range ips {
			if err = remnaExecute(ctx, a, func(callCtx context.Context, client remnaClient) error {
				return client.BlockIP(callCtx, ip, nodeUUID, durationSeconds)
			}); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func (a remnaAdapter) abuseIPBanTargets(ctx context.Context, scan remnawave.ConnectionScan, wanted map[string]bool, all bool) (map[string][]string, error) {
	ips := map[string]bool{}
	for _, node := range scan.Nodes {
		if !all && !wanted[node.UUID] {
			continue
		}
		for _, item := range node.IPs {
			ips[item.IP] = true
		}
	}
	targets := map[string][]string{}
	if all {
		nodes, err := remnaCall(ctx, a, func(callCtx context.Context, client remnaClient) ([]remnawave.Node, error) {
			return client.ListNodes(callCtx)
		})
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			if node.IsDisabled || !node.IsConnected {
				continue
			}
			for ip := range ips {
				targets[node.UUID] = append(targets[node.UUID], ip)
			}
		}
		if len(targets) == 0 && len(ips) > 0 {
			return nil, fmt.Errorf("no active abuse nodes are available")
		}
		return targets, nil
	}
	for _, node := range scan.Nodes {
		if !wanted[node.UUID] {
			continue
		}
		for _, item := range node.IPs {
			targets[node.UUID] = append(targets[node.UUID], item.IP)
		}
	}
	if len(targets) == 0 && len(ips) > 0 {
		return nil, fmt.Errorf("no affected abuse nodes are available")
	}
	return targets, nil
}
