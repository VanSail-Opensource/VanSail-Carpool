package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"net/netip"
	"time"
)

const FirewallJob = "xnode_firewall"

type FirewallMember struct {
	Epoch                                          uint64
	NodeID                                         string
	Applied, SourceApplied, Version, SourceVersion uint64
	Force                                          bool
}
type FirewallBanView struct {
	wire.FirewallBan
	IP           string    `json:"ip"`
	Rule         string    `json:"rule"`
	Count        string    `json:"count"`
	DetectedAt   time.Time `json:"detectedAt"`
	PendingNodes int       `json:"pendingNodes"`
}
type FirewallGrant struct {
	ID         string    `json:"id"`
	IP         string    `json:"ip"`
	Until      time.Time `json:"until"`
	PriceMinor int64     `json:"priceMinor,string"`
	Hours      int       `json:"hours"`
}
type FirewallQuote struct {
	WhitelistMode bool   `json:"whitelistMode"`
	Enabled       bool   `json:"enabled"`
	Price         string `json:"price"`
	Hours         int    `json:"hours"`
}
type FirewallPurchase struct {
	IP    string `json:"ip"`
	Key   string `json:"-"`
	Price string `json:"price"`
	Hours int    `json:"hours"`
}
type FirewallDetectionSink interface {
	SetDetectedPrefixes([]string)
}
type firewallRepository interface {
	JoinFirewallParticipants(context.Context, string, []wire.FirewallBan, []wire.FirewallSource) error
	FirewallRevision(context.Context) (uint64, error)
	ReconcileFirewallSources(context.Context, []string, time.Time) error
	ReconcileFirewallMember(context.Context, string, bool, bool, uint64) error
	FirewallMember(context.Context, string) (FirewallMember, error)
	RecordFirewallEvents(context.Context, string, string, []wire.FirewallEvent, wire.FirewallPolicy, time.Time, ...[]string) error
	FirewallPage(context.Context, uint64, uint64, bool, string, int) (wire.FirewallUpdate, string, error)
	MarkFirewallDelivered(context.Context, string, uint64, bool, bool, uint64) error
	SweepFirewall(context.Context, time.Time) error
	FirewallBans(context.Context, int) ([]FirewallBanView, error)
	FirewallPrefixes(context.Context, func([]string) error) error
	PurchaseFirewallExemption(context.Context, string, FirewallPurchase, int64, time.Time, ...bool) (FirewallGrant, error)
}

func FirewallPublicIP(raw string) (string, error) {
	ip, err := netip.ParseAddr(raw)
	if err != nil || ip.Zone() != "" {
		return "", ErrInvalid
	}
	ip = ip.Unmap()
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return "", ErrInvalid
	}
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
		return "", ErrInvalid
	}
	// Shared address space, documentation and benchmarking ranges are not public.
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"} {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return "", ErrInvalid
		}
	}
	return ip.String(), nil
}
