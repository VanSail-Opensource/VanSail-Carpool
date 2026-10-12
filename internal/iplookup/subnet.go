package iplookup

import (
	"context"
	"net/netip"
	"strings"
)

func joinEvidence(values []string) string { return strings.Join(values, ", ") }
func riskFlags(p ProviderResult) bool {
	return (p.Signals.Abuse != nil && *p.Signals.Abuse) || (p.Signals.Proxy != nil && *p.Signals.Proxy) || (p.Signals.VPN != nil && *p.Signals.VPN) || (p.Signals.Tor != nil && *p.Signals.Tor)
}
func flagsComplete(p ProviderResult) bool {
	return p.Signals.Abuse != nil && p.Signals.Proxy != nil && p.Signals.VPN != nil && p.Signals.Tor != nil
}
func (s *EnhancedService) checkSubnet(ctx context.Context, r *Report, c Config, save func() error) error {
	address, err := netip.ParseAddr(r.IP)
	if err != nil || address.Zone() != "" {
		return &CodeError{"IP_LOOKUP_INVALID_IP"}
	}
	address = address.Unmap()
	prefix := blockPrefix(address.String())
	if address.Is6() {
		// Live block evidence is independent; never collect it from a neighbor scan.
		r.Subnet = &SubnetResult{Prefix: prefix, Status: "skipped", Reason: "ipv6", Addresses: []SubnetAddress{}}
		return save()
	}
	if r.Subnet == nil || r.Subnet.Status == "skipped" {
		pool, err := s.pool(ctx, c)
		if err != nil {
			return err
		}
		if err := initializeSubnet(ctx, r, pool); err != nil {
			return err
		}
		if err := save(); err != nil {
			return err
		}
	}
	if len(r.Refusals) > 0 {
		finishInventorySubnet(r, "ip_not_passed")
		return save()
	}
	// An earlier /24 AbuseIPDB block check that reported zero abusive
	// addresses already covers the abuse item of this scan: skip it instead
	// of re-evaluating every neighbor, while proxy membership and the
	// per-address IPAPI checks continue unchanged.
	if abuseBlockClean(r, prefix) {
		for i := range r.Subnet.Addresses {
			if r.Subnet.Addresses[i].Abuse == "pending" {
				r.Subnet.Addresses[i].Abuse = "skipped"
			}
		}
	}
	needBlock := false
	for _, row := range r.Subnet.Addresses {
		needBlock = needBlock || row.Abuse == "pending"
	}
	if needBlock {
		block := s.reportBlock(ctx, r, c)
		if r.LiveDetails != nil {
			r.LiveDetails.Block = block
		}
		reported := map[string]bool{}
		for _, item := range block.Addresses {
			if item.Reports > 0 || item.Score > 0 {
				reported[item.IP] = true
			}
		}
		for i := range r.Subnet.Addresses {
			row := &r.Subnet.Addresses[i]
			if row.Status == "flagged" {
				continue
			}
			row.Abuse = block.Status
			if block.MinAddress != "" && block.MaxAddress != "" {
				addr, _ := netip.ParseAddr(row.IP)
				minimum, e1 := netip.ParseAddr(block.MinAddress)
				maximum, e2 := netip.ParseAddr(block.MaxAddress)
				if e1 != nil || e2 != nil || addr.Compare(minimum) < 0 || addr.Compare(maximum) > 0 {
					row.Abuse = "partial"
				}
			}
			if reported[row.IP] {
				row.Status = "flagged"
				row.Abuse = "flagged"
				row.IPAPI = "skipped"
				row.Sources = append(row.Sources, "abuseipdb")
			}
		}
		if err := save(); err != nil {
			return err
		}
	}
	provider := s.ipapi
	config := ProviderConfig{ID: "ipapi"}
	for _, p := range c.Providers {
		if p.ID == "ipapi" {
			config = p
		}
	}
	quotaLimited := false
	for i := range r.Subnet.Addresses {
		row := &r.Subnet.Addresses[i]
		if row.Status == "flagged" || row.IPAPI != "pending" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		result := ProviderResult{}
		var callErr error
		if !config.Enabled || quotaLimited {
			callErr = &CodeError{"IP_LOOKUP_PROVIDER_QUOTA"}
		} else if row.IP == r.IP && r.Checkpoint.Enhanced.IPAPI != nil {
			result = *r.Checkpoint.Enhanced.IPAPI
		} else {
			result, callErr = provider.Lookup(ctx, row.IP, config)
		}
		row.IPAPI = "error"
		if callErr == nil {
			if riskFlags(result) {
				row.Status = "flagged"
				row.IPAPI = "flagged"
				row.Sources = append(row.Sources, "ipapi")
			} else if flagsComplete(result) {
				row.IPAPI = "success"
				// A skipped abuse item means the earlier clean /24 block
				// check already covered this address.
				if row.Proxy == "success" && (row.Abuse == "success" || row.Abuse == "skipped") {
					row.Status = "clean"
				}
			} else {
				row.IPAPI = "partial"
			}
		}
		if ErrorCode(callErr) == "IP_LOOKUP_PROVIDER_QUOTA" {
			quotaLimited = true
		}
		if err := save(); err != nil {
			return err
		}
	}
	r.Subnet.Status = "success"
	for _, row := range r.Subnet.Addresses {
		if row.Status == "unknown" {
			r.Subnet.Status = "partial"
		}
	}
	return save()
}
