package app

import (
	"context"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// RequestNativeConnectionScan retains only the target reference in the durable job.
func (a proxyProvider) RequestNativeConnectionScan(ctx context.Context, userID string) (string, error) {
	identity, err := a.native.IdentityForUser(ctx, userID)
	if err != nil {
		return "", err
	}
	nonce, err := ids.Token(16)
	if err != nil {
		return "", err
	}
	return "xna:" + strconv.FormatInt(identity.ProxyUserID, 10) + ":" + nonce, nil
}

func (a proxyProvider) pollNativeConnectionScan(ctx context.Context, jobID string) (connections.ProviderScan, error) {
	result := connections.ProviderScan{Completed: true, ProgressPercent: 100, Nodes: []connections.ProviderNode{}}
	parts := strings.Split(jobID, ":")
	if len(parts) != 3 {
		return result, xnode.ErrInvalid
	}
	identity, err := a.native.Identity(ctx, parts[1])
	if err != nil {
		return result, err
	}
	nodes, err := a.master.UserNodeMetrics(ctx, identity.UserID)
	if err != nil {
		return result, err
	}
	for _, node := range nodes {
		item := connections.ProviderNode{UUID: node.NodeID, Name: node.Name, CountryCode: node.CountryCode, Unavailable: !node.Available, IPs: []connections.Observation{}}
		if node.Available {
			ips, err := a.memberNodeIPs(ctx, identity.MemberOwnerID(), node.NodeID)
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			if err == xnode.ErrInvalid {
				return result, err
			}
			if err != nil {
				item.Unavailable = true
			} else {
				item.IPs = ips
			}
		}

		result.Nodes = append(result.Nodes, item)
	}
	// Discard observations if ownership/freshness changed while queued work ran.
	current, err := a.master.UserNodeMetrics(ctx, identity.UserID)
	if err != nil {
		return result, err
	}
	available := map[string]bool{}
	for _, node := range current {
		available[node.NodeID] = node.Available
	}
	for i := range result.Nodes {
		if !available[result.Nodes[i].UUID] {
			result.Nodes[i].Unavailable = true
			result.Nodes[i].IPs = nil
		}
	}
	return result, nil
}
