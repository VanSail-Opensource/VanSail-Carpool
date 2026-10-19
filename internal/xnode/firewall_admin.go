package xnode

import (
	"context"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/outbox"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

const FirewallExemptionOperation = "xnode_firewall_exemption"

type AdminFirewallExemption struct {
	Prefix string `json:"prefix"`
	Hours  int    `json:"hours"`
	Reason string `json:"reason,omitempty"`
	Key    string `json:"-"`
}
type adminFirewallRepository interface {
	GrantAdminFirewallExemption(context.Context, string, AdminFirewallExemption, time.Time) (model.OperationReceipt, error)
	FinalizeAdminFirewallExemption(context.Context, string, time.Time) (bool, error)
}

func (s *Service) GrantFirewallExemption(ctx context.Context, actor string, input AdminFirewallExemption) (model.OperationReceipt, error) {
	prefix, err := wire.FirewallPrefix(strings.TrimSpace(input.Prefix))
	if err != nil || input.Hours < 1 || input.Hours > 8760 || utf8.RuneCountInString(input.Reason) > 1000 || input.Key == "" || len(input.Key) > 128 {
		return model.OperationReceipt{}, ErrInvalid
	}
	if !validAdminExemption(prefix, s.Now()) {
		return model.OperationReceipt{}, ErrInvalid
	}
	if prefix.Bits() != prefix.Addr().BitLen() && s.master != nil {
		nodes, err := s.Nodes(ctx)
		if err != nil {
			return model.OperationReceipt{}, err
		}
		for _, node := range nodes {
			if !node.Enabled || !s.master.Connected(node.ID) {
				continue
			}
			owned, err := s.NativeOwner(ctx, node.ID)
			if err != nil {
				return model.OperationReceipt{}, err
			}
			if !owned {
				continue
			}
			_, settings, err := s.Node(ctx, node.ID)
			if err != nil {
				return model.OperationReceipt{}, err
			}
			if settings.Firewall.Enabled && !containsCapability(s.master.Capabilities(node.ID), "firewall_cidr_exemptions_v1") {
				return model.OperationReceipt{}, &CapabilityError{Capability: "firewall_cidr_exemptions_v1"}
			}
		}
	}
	input.Prefix = prefix.String()
	input.Reason = strings.TrimSpace(input.Reason)
	repo, ok := s.repo.(adminFirewallRepository)
	if !ok {
		return model.OperationReceipt{}, ErrUnavailable
	}
	return repo.GrantAdminFirewallExemption(ctx, actor, input, s.Now())
}
func (s *Service) handleFirewallExemption(ctx context.Context, operation providerops.Operation) error {
	repo, ok := s.repo.(adminFirewallRepository)
	if !ok {
		return ErrUnavailable
	}
	done, err := repo.FinalizeAdminFirewallExemption(ctx, operation.Receipt.ID, s.Now())
	if err != nil {
		return err
	}
	if !done {
		return outbox.ErrBusy
	}
	return nil
}

// validAdminExemption mirrors the pinned agent's public-address restriction for
// exact addresses and applies the same restriction to administrator prefixes.
func validAdminExemption(prefix netip.Prefix, now time.Time) bool {
	ip := prefix.Addr()
	if !prefix.IsValid() || ip.Zone() != "" || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return false
	}
	if prefix.Bits() == ip.BitLen() {
		return wire.FirewallExemption{IP: ip.String(), Generation: "admin", Until: now.Add(time.Hour)}.Validate() == nil
	}
	return true
}
