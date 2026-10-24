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
		ips, err := a.memberNodeIPs(ctx, identity.MemberOwnerID(), node.ID)
		if err != nil {
			return result, err
		}
		item := connections.ProviderNode{UUID: node.ID, Name: node.Name, CountryCode: node.CountryCode, IPs: ips}

		result.Nodes = append(result.Nodes, item)
	}
	return result, nil
}
func (a proxyProvider) ControlMemberIP(ctx context.Context, user, ip, node string, drop bool, seconds int, unblock bool) error {
	_, err := a.native.IdentityForUser(ctx, user)
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
	principals, err := a.native.MemberPrincipals(ctx, user, node)
	if err != nil {
		return err
	}
	for _, principal := range principals {
		if _, err = a.master.Command(ctx, node, "", "ips.control", wire.IPControl{UserID: int(principal), SourceIP: ip, Drop: drop, BlockSeconds: seconds, Unblock: unblock}); err != nil {
			return err
		}
	}
	return nil
}
func (a proxyProvider) DropConnection(context.Context, string, string) error { return xnode.ErrInvalid }
func (a proxyProvider) BlockIP(context.Context, string, string, int) error   { return xnode.ErrInvalid }
func (a proxyProvider) UnblockIP(context.Context, string, string) error      { return xnode.ErrInvalid }

// The caller supplies the authenticated member. No principal selector comes
// from browser input, and observations stay request-local.
func (a proxyProvider) memberNodeIPs(ctx context.Context, user, node string) ([]connections.Observation, error) {
	principals, err := a.native.MemberPrincipals(ctx, user, node)
	if err != nil {
		return nil, err
	}
	out := []connections.Observation{}
	seen := map[string]bool{}
	for _, principal := range principals {
		raw, err := a.master.Command(ctx, node, "", "users.active_ips", map[string]int64{"user_id": principal})
		if err != nil {
			return nil, err
		}
		var ips []struct {
			UserID      int64  `json:"user_id"`
			SourceIP    string `json:"source_ip"`
			Connections int    `json:"connections"`
		}
		if json.Unmarshal(raw, &ips) != nil {
			return nil, xnode.ErrInvalid
		}
		for _, ip := range ips {
			if ip.UserID != principal || ip.Connections < 0 {
				return nil, xnode.ErrInvalid
			}
			if ip.Connections > 0 && !seen[ip.SourceIP] {
				seen[ip.SourceIP] = true
				out = append(out, connections.Observation{Address: ip.SourceIP, LastSeen: time.Now().UTC()})
			}
		}
	}
	return out, nil
}
