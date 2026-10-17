package xnode

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"strings"
)

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
	return repo.PurchaseFirewallExemption(ctx, user, input, price, s.Now())
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
		return ErrInvalid
	}
	var report wire.FirewallEvents
	if err := wire.Decode(message.Data, &report); err != nil || len(report.Events) > 256 {
		return ErrInvalid
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
	_, settings, err := m.service.Node(ctx, peer.nodeID)
	if err != nil {
		return err
	}
	repo, ok := m.service.repo.(firewallRepository)
	if !ok {
		return ErrUnavailable
	}
	if err = repo.RecordFirewallEvents(ctx, peer.nodeID, report.InstanceID, report.Events, settings.Firewall, m.service.Now()); err != nil {
		return err
	}
	ids := make([]string, len(report.Events))
	for i, e := range report.Events {
		ids[i] = e.ID
	}
	m.NotifyFirewallSources()
	return peer.send(ctx, wire.New("firewall.events.ack", "", 0, wire.FirewallEventAck{IDs: ids}))
}
