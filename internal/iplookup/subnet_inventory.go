package iplookup

import (
	"context"
	"net/netip"
)

// initializeSubnet records all inventory matches even when later checks refuse the IP.
func initializeSubnet(ctx context.Context, r *Report, pool *proxyPool) error {
	address, err := netip.ParseAddr(r.IP)
	if err != nil || address.Zone() != "" {
		return &CodeError{"IP_LOOKUP_INVALID_IP"}
	}
	address = address.Unmap()
	prefix := blockPrefix(address.String())
	if address.Is6() {
		r.Subnet = &SubnetResult{Prefix: prefix, Status: "skipped", Reason: "ipv6", Addresses: []SubnetAddress{}}
		return nil
	}
	r.Subnet = &SubnetResult{Prefix: prefix, Status: "processing", Addresses: make([]SubnetAddress, 0, 256), Sources: pool.metadata()}
	network, _ := netip.ParsePrefix(prefix)
	for ip := network.Addr(); network.Contains(ip); ip = ip.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		row := SubnetAddress{IP: ip.String(), Status: "unknown", Proxy: "success", Abuse: "pending", IPAPI: "pending", Sources: []string{}}
		if !pool.complete {
			row.Proxy = "partial"
		}
		if ids := pool.matchingSources(ip); len(ids) > 0 {
			row.Status, row.Abuse, row.IPAPI = "flagged", "skipped", "skipped"
			for _, id := range ids {
				for _, source := range pool.sources {
					if source.ID != id {
						continue
					}
					if sourceCategory(source.Category) == AbuseCategory {
						row.Abuse = "flagged"
					} else {
						row.Proxy = "flagged"
					}
				}
			}
			row.Sources = ids
		}
		r.Subnet.Addresses = append(r.Subnet.Addresses, row)
	}
	return nil
}

// finishInventorySubnet skips only external work and retains completed membership evidence.
func finishInventorySubnet(r *Report, reason string) {
	if r.Subnet == nil || r.Subnet.Reason == "ipv6" {
		return
	}
	r.Subnet.Status, r.Subnet.Reason = "success", reason
	for i := range r.Subnet.Addresses {
		row := &r.Subnet.Addresses[i]
		if row.Abuse == "pending" {
			row.Abuse = "skipped"
		}
		if row.IPAPI == "pending" {
			row.IPAPI = "skipped"
		}
		if row.Status != "flagged" && row.Proxy == "success" {
			row.Status = "clean"
		}
		if row.Proxy == "partial" {
			r.Subnet.Status = "partial"
		}
	}
}
