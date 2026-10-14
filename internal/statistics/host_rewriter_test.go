package statistics

import (
	"context"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

func TestReplaceFirstStandaloneMultiplier(t *testing.T) {
	tests := []struct {
		name                     string
		input, replacement, want string
		changed                  bool
	}{
		{"first marker only", "HK 1.0x premium 2.0x", "1.5", "HK 1.5x premium 2.0x", true},
		{"Chinese suffix", "0.1x测试", "1.5", "1.5x测试", true},
		{"Chinese surrounding", "测试0.1x测试", "1.5", "测试1.5x测试", true},
		{"Chinese prefix", "测试0.1x", "1.5", "测试1.5x", true},
		{"unmarked decimal", "HK 1.0 premium", "1.5", "HK 1.0 premium", false},
		{"uppercase marker", "0.1X测试", "1.5", "0.1X测试", false},
		{"embedded ASCII", "prefix0.1x suffix", "2", "prefix0.1x suffix", false},
		{"suffix embedded ASCII", "version 1.0xbuild", "2", "version 1.0xbuild", false},
		{"IP-like value", "node 10.1.2.3x", "2", "node 10.1.2.3x", false},
		{"no multiplier", "no multiplier", "2", "no multiplier", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, changed := replaceFirstMultiplier(test.input, test.replacement)
			if got != test.want || changed != test.changed {
				t.Fatalf("replaceFirstMultiplier(%q) = %q,%v; want %q,%v", test.input, got, changed, test.want, test.changed)
			}
		})
	}
}

func TestQueueHostMultiplierUpdatesAllowsZeroMultiplier(t *testing.T) {
	t.Parallel()
	provider := &statisticsProviderStub{
		nodes: []Node{{UUID: "node-1", Multiplier: 0}},
		hosts: []Host{
			{UUID: "host-1", Remark: "Edge 1.0x", Nodes: []string{"node-1"}},
			{UUID: "host-many", Remark: "Edge 1.0x", Nodes: []string{"node-1", "node-2"}},
		},
	}
	repository := &hostUpdateRepositoryStub{}
	err := QueueHostMultiplierUpdates(context.Background(), provider, repository, "admin-1", time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("QueueHostMultiplierUpdates(): %v", err)
	}
	if len(repository.inputs) != 1 {
		t.Fatalf("queued updates = %d, want one single-link host", len(repository.inputs))
	}
	target, err := decodeHostRemarkTarget(repository.inputs[0].SealedTarget)
	if err != nil || target.HostUUID != "host-1" || target.Remark != "Edge 0.0x" {
		t.Fatalf("queued target = (%+v, %v)", target, err)
	}
}

func TestIntegerMultiplierFormattingRetainsLowercaseMarker(t *testing.T) {
	t.Parallel()
	first, ok := formatMultiplierToken(2)
	if !ok || first != "2.0" {
		t.Fatalf("formatMultiplierToken(2) = %q,%v", first, ok)
	}
	remark, changed := replaceFirstMultiplier("Edge 1.0x", first)
	if !changed || remark != "Edge 2.0x" {
		t.Fatalf("first rewrite = %q,%v", remark, changed)
	}
	second, _ := formatMultiplierToken(3)
	remark, changed = replaceFirstMultiplier(remark, second)
	if !changed || remark != "Edge 3.0x" {
		t.Fatalf("second rewrite = %q,%v", remark, changed)
	}
}

type hostUpdateRepositoryStub struct {
	inputs []providerops.CreateInput
}

func (r *hostUpdateRepositoryStub) CreateProviderOperation(_ context.Context, input providerops.CreateInput, _ time.Time) (providerops.Operation, bool, error) {
	r.inputs = append(r.inputs, input)
	return providerops.Operation{}, true, nil
}
