package xnode

import (
	"net/netip"
	"slices"
	"strconv"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

// CompileFirewallFilter validates bounded expressions before evaluating records.
func CompileFirewallFilter(root FirewallFilter) (func(FirewallRecord) bool, error) {
	nodes, conditions := 0, 0
	var compile func(FirewallFilter, int) (func(FirewallRecord) bool, error)
	compile = func(filter FirewallFilter, depth int) (func(FirewallRecord) bool, error) {
		nodes++
		if nodes > 64 || depth > 4 {
			return nil, ErrInvalid
		}
		if filter.Op != "" {
			if filter.Op != "and" && filter.Op != "or" || filter.Field != "" || filter.Value != "" || filter.From != "" || filter.To != "" {
				return nil, ErrInvalid
			}
			children := make([]func(FirewallRecord) bool, 0, len(filter.Children))
			for _, child := range filter.Children {
				predicate, err := compile(child, depth+1)
				if err != nil {
					return nil, err
				}
				children = append(children, predicate)
			}
			return func(record FirewallRecord) bool {
				for _, child := range children {
					if child(record) == (filter.Op == "or") {
						return filter.Op == "or"
					}
				}
				return filter.Op == "and"
			}, nil
		}
		conditions++
		if conditions > 32 || len(filter.Children) != 0 || len(filter.Value) > 128 || len(filter.From) > 64 || len(filter.To) > 64 {
			return nil, ErrInvalid
		}
		if filter.Field != "detectedAt" && (filter.From != "" || filter.To != "") {
			return nil, ErrInvalid
		}
		switch filter.Field {
		case "ip":
			prefix, err := ExemptionPrefix(filter.Value)
			if err != nil {
				return nil, ErrInvalid
			}
			return func(record FirewallRecord) bool {
				current, err := netip.ParsePrefix(record.Prefix)
				return err == nil && current.Overlaps(prefix)
			}, nil
		case "rule":
			if !slices.Contains(wire.FirewallRuleNames(), filter.Value) {
				return nil, ErrInvalid
			}
			return func(record FirewallRecord) bool { return record.Rule == filter.Value }, nil
		case "status":
			if filter.Value != "banned" && filter.Value != "reported" {
				return nil, ErrInvalid
			}
			return func(record FirewallRecord) bool { return record.Banned == (filter.Value == "banned") }, nil
		case "hits":
			minimum, err := strconv.ParseUint(filter.Value, 10, 64)
			if err != nil {
				return nil, ErrInvalid
			}
			return func(record FirewallRecord) bool {
				count, err := strconv.ParseUint(record.Count, 10, 64)
				return err == nil && count >= minimum
			}, nil
		case "detectedAt":
			if filter.Value != "" || filter.From == "" && filter.To == "" {
				return nil, ErrInvalid
			}
			var from, to time.Time
			var err error
			if filter.From != "" {
				from, err = time.Parse(time.RFC3339Nano, filter.From)
				if err != nil {
					return nil, ErrInvalid
				}
			}
			if filter.To != "" {
				to, err = time.Parse(time.RFC3339Nano, filter.To)
				if err != nil {
					return nil, ErrInvalid
				}
			}
			if !from.IsZero() && !to.IsZero() && from.After(to) {
				return nil, ErrInvalid
			}
			return func(record FirewallRecord) bool {
				return (from.IsZero() || !record.DetectedAt.Before(from)) && (to.IsZero() || !record.DetectedAt.After(to))
			}, nil
		default:
			return nil, ErrInvalid
		}
	}
	if root.Op == "" {
		return nil, ErrInvalid
	}
	return compile(root, 1)
}
