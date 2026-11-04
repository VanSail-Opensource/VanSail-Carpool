package app

import (
	"context"

	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connections"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"strings"
	"time"
)

func (a proxyProvider) RequestConnectionScan(context.Context, string) (string, error) {
	return "", xnode.ErrInvalid
}
func (a proxyProvider) PollConnectionScan(ctx context.Context, id string) (connections.ProviderScan, error) {
	if !strings.HasPrefix(id, "xna:") {
		return connections.ProviderScan{}, xnode.ErrInvalid
	}
	return a.pollNativeConnectionScan(ctx, id)
}
func (a proxyProvider) ControlMemberIP(ctx context.Context, user, ip, node string, drop bool, seconds int, unblock bool) error {
	if drop || seconds != 0 || !unblock {
		return xnode.ErrInvalid
	}
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
