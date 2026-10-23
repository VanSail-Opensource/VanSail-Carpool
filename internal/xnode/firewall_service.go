package xnode

import (
	"context"
	"errors"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type firewallEventFrameError struct {
	code  string
	cause error
}

func (e firewallEventFrameError) Error() string          { return e.code }
func (e firewallEventFrameError) Unwrap() error          { return e.cause }
func (e firewallEventFrameError) frameErrorCode() string { return e.code }

func firewallEventFailure(code string, cause error) error {
	return firewallEventFrameError{code: code, cause: cause}
}

func safeFrameErrorCode(err error) string {
	var coded interface{ frameErrorCode() string }
	if errors.As(err, &coded) {
		return coded.frameErrorCode()
	}
	return "XNODE_FRAME_REJECTED"
}

func hasFirewallCapability(capabilities []string) bool {
	for _, c := range capabilities {
		if c == "firewall_control_v1" {
			return true
		}
	}
	return false
}
func firewallForCapabilities(config wire.Config, capabilities []string) wire.Config {
	// Keep the cached validated source immutable. Older agents cannot safely
	// detect autonomously, but still receive revocations and other configuration.
	config.Firewall.MasterControlled = hasFirewallCapability(capabilities)
	if !config.Firewall.MasterControlled {
		config.Firewall.Protect = false
		config.Firewall.Rules = nil
		config.Firewall.BanSeconds = 0
	}
	return config
}
func (s *Service) FirewallQuote(ctx context.Context) (FirewallQuote, error) {
	config, err := s.Global(ctx)
	if err != nil {
		return FirewallQuote{}, err
	}
	price, e := model.ParseTXBMajor(config.FirewallUnbanPrice)
	return FirewallQuote{Enabled: e == nil && price > 0 && config.FirewallExemptionHours > 0, Price: config.FirewallUnbanPrice, Hours: config.FirewallExemptionHours}, nil
}

func (s *Service) OwnedFirewallQuote(ctx context.Context, user string) (FirewallQuote, error) {
	quote, err := s.FirewallQuote(ctx)
	if err != nil {
		return quote, err
	}
	_, settings, err := s.readDIYSettings(ctx, user)
	if err != nil {
		return quote, err
	}
	quote.WhitelistMode = settings.Whitelist.Enabled
	quote.Enabled = quote.Enabled && !quote.WhitelistMode
	return quote, nil
}
func (s *Service) PurchaseFirewallExemption(ctx context.Context, user string, input FirewallPurchase) (FirewallGrant, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ip, err := FirewallPublicIP(strings.TrimSpace(input.IP))
	if err != nil {
		return FirewallGrant{}, err
	}
	input.IP = ip
	if len(input.Key) < 16 || len(input.Key) > 128 {
		return FirewallGrant{}, ErrInvalid
	}
	quote, err := s.FirewallQuote(ctx)
	if err != nil {
		return FirewallGrant{}, err
	}
	if !quote.Enabled || input.Price != quote.Price || input.Hours != quote.Hours {
		return FirewallGrant{}, ErrConflict
	}
	price, err := model.ParseTXBMajor(quote.Price)
	if err != nil {
		return FirewallGrant{}, ErrInvalid
	}
	repo, ok := s.repo.(firewallRepository)
	if !ok {
		return FirewallGrant{}, ErrUnavailable
	}
	_, settings, err := s.readDIYSettings(ctx, user)
	if err != nil {
		return FirewallGrant{}, err
	}
	return repo.PurchaseFirewallExemption(ctx, user, input, price, s.Now(), !settings.Whitelist.Enabled)
}
func (s *Service) FirewallBans(ctx context.Context) ([]FirewallBanView, error) {
	repo, ok := s.repo.(firewallRepository)
	if !ok {
		return nil, ErrUnavailable
	}
	return repo.FirewallBans(ctx, 200)
}
func (m *Master) SetFirewallInventory(inventory FirewallInventory) { m.firewallInventory = inventory }
func (m *Master) NotifyFirewallSources() {
	select {
	case m.firewallWake <- struct{}{}:
	default:
	}
}
func (m *Master) receiveFirewallEvents(ctx context.Context, peer *agentPeer, message wire.Message) error {
	if !hasFirewallCapability(m.Capabilities(peer.nodeID)) {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_CAPABILITY_INVALID", ErrInvalid)
	}
	var report wire.FirewallEvents
	if err := wire.Decode(message.Data, &report); err != nil {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_FRAME_INVALID", err)
	}
	if len(report.Events) > wire.MaxFirewallEventsPerFrame {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_FRAME_OVERSIZED", ErrInvalid)
	}
	if report.State != nil {
		copy := *report.State
		m.mu.Lock()
		live := m.live[peer.nodeID]
		live.firewall = &copy
		m.live[peer.nodeID] = live
		m.mu.Unlock()
	}
	if len(report.Events) == 0 {
		return nil
	}
	node, settings, err := m.service.Node(ctx, peer.nodeID)
	if err != nil {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_SETTINGS_FAILED", err)
	}
	global, err := m.service.Global(ctx)
	if err != nil {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_SETTINGS_FAILED", err)
	}
	settings.Firewall = global.FirewallDetection.Apply(settings.Firewall)
	repo, ok := m.service.repo.(firewallRepository)
	if !ok {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_STORAGE_FAILED", ErrUnavailable)
	}
	owned, err := m.service.NativeOwner(ctx, peer.nodeID)
	if err != nil {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_SETTINGS_FAILED", err)
	}
	if owned && node.Enabled && settings.Firewall.Enabled {
		// Durable events can precede synchronized telemetry and the periodic join.
		// Register without waiting for agent commands on this socket reader.
		if err = repo.ReconcileFirewallMember(ctx, peer.nodeID, true, false, 0); err != nil {
			return firewallEventFailure("XNODE_FIREWALL_EVENT_STORAGE_FAILED", err)
		}
	} else {
		// Drain old journal events without authorizing new restrictions.
		settings.Firewall.Protect = false
	}
	if err = repo.RecordFirewallEvents(ctx, peer.nodeID, report.InstanceID, report.Events, settings.Firewall, m.service.Now(), m.protectedIPs(true)); err != nil {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_STORAGE_FAILED", err)
	}
	ids := make([]string, len(report.Events))
	for i, e := range report.Events {
		ids[i] = e.ID
	}
	m.NotifyFirewallSources()
	if err = peer.send(ctx, wire.New("firewall.events.ack", "", 0, wire.FirewallEventAck{IDs: ids})); err != nil {
		return firewallEventFailure("XNODE_FIREWALL_EVENT_ACK_FAILED", err)
	}
	return nil
}
