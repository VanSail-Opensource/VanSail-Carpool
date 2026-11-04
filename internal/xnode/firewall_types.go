package xnode

import (
	"context"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
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
}
