package botcommands

import (
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestFormatBytesMatchesWebUnits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value int64
		want  string
	}{
		{value: 0, want: "0 GB"},
		{value: 1, want: "1 B"},
		{value: 1024, want: "1 KB"},
		{value: 1536, want: "1.5 KB"},
		{value: 1280, want: "1.25 KB"},
		{value: 1 << 20, want: "1 MB"},
		{value: 1 << 30, want: "1 GB"},
		{value: 1 << 40, want: "1 TB"},
		{value: 1 << 50, want: "1 PB"},
	}
	for _, test := range tests {
		if got := formatBytes(test.value); got != test.want {
			t.Errorf("formatBytes(%d) = %q, want %q", test.value, got, test.want)
		}
	}
}

func TestParseBytesRejectsMalformedValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{"", "-1", "1.5", "bytes", "9223372036854775808"} {
		if got := parseBytes(value); got != 0 {
			t.Errorf("parseBytes(%q) = %d, want 0", value, got)
		}
	}
}

func TestUsageAndNodeBarsClampAndAllocate(t *testing.T) {
	t.Parallel()
	if got := percentage(200, 100); got != 100 {
		t.Fatalf("percentage() = %.1f, want 100", got)
	}
	if got := usageBar(3); got != strings.Repeat("⬜", 8) {
		t.Fatalf("usageBar(3) = %q", got)
	}
	nodes := []displayNode{{Bytes: 50}, {Bytes: 30}, {Bytes: 20}}
	counts := distributeNodeCells(nodes, 100)
	if len(counts) != 3 || counts[0] != 15 || counts[1] != 9 || counts[2] != 6 {
		t.Fatalf("distributeNodeCells() = %v, want [15 9 6]", counts)
	}
	if len([]rune(nodeDistribution(nodes, 100))) != nodeBarWidth {
		t.Fatal("node distribution is not 30 cells")
	}
}

func TestFormatSubscriptionUsesDisplayedNodeTotal(t *testing.T) {
	t.Parallel()
	nodes := make([]model.TopNode, 0, 6)
	for _, value := range []string{"50", "30", "20", "10", "5", "1000"} {
		nodes = append(nodes, model.TopNode{Name: "node", CountryCode: "us", TotalBytes: value})
	}
	dashboard := model.Dashboard{ActivePurchase: &model.Purchase{}, Statistics: &model.Statistics{
		UsedTrafficBytes: "200", TrafficLimitBytes: "100", TopNodes: nodes,
	}}
	displayed, total := displayNodes(nodes)
	if len(displayed) != 5 || total != 115 {
		t.Fatalf("displayNodes() = (%d, %d), want (5, 115)", len(displayed), total)
	}
	message := FormatSubscription(Text(English), dashboard, time.Now().UTC())
	if !strings.Contains(message, strings.Repeat("🟩", 8)+" 100%") || !strings.Contains(message, `\(US\)`) {
		t.Fatalf("FormatSubscription() = %q", message)
	}
	if !strings.Contains(message, `\(43\.5%\)`) {
		t.Fatal("node shares did not use the displayed five-node total")
	}
}

func TestFormatSubscriptionOmitsEmptyDistributionAndLimitsOutput(t *testing.T) {
	t.Parallel()
	dashboard := model.Dashboard{ActivePurchase: &model.Purchase{}, Statistics: &model.Statistics{
		UsedTrafficBytes: "bad", TrafficLimitBytes: "0", TopNodes: []model.TopNode{{Name: "empty", TotalBytes: "0"}},
	}}
	message := FormatSubscription(Text(English), dashboard, time.Now().UTC())
	if strings.Contains(message, "\\[") {
		t.Fatalf("empty distribution was rendered: %q", message)
	}
	dashboard.Statistics.TopNodes[0] = model.TopNode{Name: strings.Repeat("x", MessageLimit*2), CountryCode: "us", TotalBytes: "1"}
	message = FormatSubscription(Text(English), dashboard, time.Now().UTC())
	if len([]rune(message)) > MessageLimit || !strings.HasSuffix(message, markdownV2Ellipsis) {
		t.Fatalf("bounded subscription has %d runes", len([]rune(message)))
	}
}
