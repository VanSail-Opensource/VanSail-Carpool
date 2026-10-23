package app

import (
	"context"

	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"strings"
	"time"
)

func (a proxyProvider) RequestConnectionScan(ctx context.Context, id string) (string, error) {
	if _, err := a.native.Identity(ctx, id); err != nil {
		return "", err
	}
	nonce, err := ids.Token(16)
	if err != nil {
		return "", err
	}
	return "xn:" + id + "::" + nonce, nil
}
func (a proxyProvider) PollConnectionScan(ctx context.Context, id string) (connections.ProviderScan, error) {
	if strings.HasPrefix(id, "xna:") {
		return a.pollNativeConnectionScan(ctx, id)
	}
	if !strings.HasPrefix(id, "xn:") {
		return connections.ProviderScan{}, xnode.ErrInvalid
	}
	parts := strings.Split(id, ":")
	if len(parts) != 4 {
		return connections.ProviderScan{}, xnode.ErrInvalid
	}
	identity, err := a.native.Identity(ctx, parts[1])
	if err != nil {
		return connections.ProviderScan{}, err
	}
	result := connections.ProviderScan{Completed: true, ProgressPercent: 100, Nodes: []connections.ProviderNode{}}
	nodes, err := a.native.Nodes(ctx)
	if err != nil {
		return result, err
	}
	for _, node := range nodes {
		owned, err := a.native.NativeOwner(ctx, node.ID)
		if err != nil {
			return result, err
		}
		if !owned || !a.master.Connected(node.ID) {
			continue
		}
		raw, err := a.master.Command(ctx, node.ID, "", "users.active_ips", map[string]int64{"user_id": identity.ProxyUserID})
		if err != nil {
			return result, err
		}
		var ips []struct {
			UserID      int64  `json:"user_id"`
			SourceIP    string `json:"source_ip"`
			Connections int    `json:"connections"`
		}
		if json.Unmarshal(raw, &ips) != nil {
			return result, xnode.ErrInvalid
		}
		item := connections.ProviderNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, IPs: []connections.Observation{}}
		for _, ip := range ips {
			if ip.UserID != identity.ProxyUserID || ip.Connections < 0 {
				return result, xnode.ErrInvalid
			}
			if ip.Connections > 0 {
				item.IPs = append(item.IPs, connections.Observation{Address: ip.SourceIP, LastSeen: time.Now().UTC()})
			}
		}
		result.Nodes = append(result.Nodes, item)
	}
	return result, nil
}
func (a proxyProvider) ControlMemberIP(ctx context.Context, user, ip, node string, drop bool, seconds int, unblock bool) error {
	identity, err := a.native.IdentityForUser(ctx, user)
	if err != nil {
		return err
	}
	owned, err := a.native.NativeOwner(ctx, node)
	if err != nil {
		return err
	}
	if !owned {
		return xnode.ErrNotFound
	}
	_, err = a.master.Command(ctx, node, "", "ips.control", wire.IPControl{UserID: int(identity.ProxyUserID), SourceIP: ip, Drop: drop, BlockSeconds: seconds, Unblock: unblock})
	return err
}
func (a proxyProvider) DropConnection(context.Context, string, string) error { return xnode.ErrInvalid }
func (a proxyProvider) BlockIP(context.Context, string, string, int) error   { return xnode.ErrInvalid }
func (a proxyProvider) UnblockIP(context.Context, string, string) error      { return xnode.ErrInvalid }
