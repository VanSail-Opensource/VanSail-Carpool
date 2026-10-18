package app

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

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
	nonce, err := ids.Token(12)
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
			// Master.Command uses the existing durable receipt and per-node executor.
			raw, err := a.master.Command(ctx, node.NodeID, "", "users.active_ips", map[string]int64{"user_id": identity.ProxyUserID})
			if ctx.Err() != nil {
				return result, ctx.Err()
			}
			if err != nil {
				item.Unavailable = true
			} else {
				var ips []struct {
					UserID      int64  `json:"user_id"`
					SourceIP    string `json:"source_ip"`
					Connections int    `json:"connections"`
				}
				if json.Unmarshal(raw, &ips) != nil {
					return result, xnode.ErrInvalid
				}
				for _, ip := range ips {
					if ip.UserID != identity.ProxyUserID || ip.Connections < 0 {
						return result, xnode.ErrInvalid
					}
					if ip.Connections > 0 {
						item.IPs = append(item.IPs, connections.Observation{Address: ip.SourceIP, LastSeen: time.Now().UTC()})
					}
				}
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
