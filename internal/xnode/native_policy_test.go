package xnode

import (
	"context"
	"encoding/json"
	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
	"testing"
	"time"
)

func TestNativeGlobalRetainsLegacyScheduleAndMasterGeoIP(t *testing.T) {
	fresh := DefaultGlobal()
	if fresh.GeodataIntervalHours == nil || *fresh.GeodataIntervalHours != 24 || fresh.SourceGeoIP.Enabled || fresh.SourceGeoIP.Selectors[0] != "geoip:cn" {
		t.Fatal("unsafe new defaults")
	}
	legacy, err := DecodeGlobal(`{"geodataCron":"0 * * * *","geoIpUrl":"https://assets.example/geoip.dat","geoSiteUrl":"https://assets.example/geosite.dat"}`)
	if err != nil || legacy.GeodataIntervalHours != nil {
		t.Fatalf("legacy schedule changed: %v", err)
	}
	native := map[string]any{}
	_, hashes, err := GlobalWire(legacy, native)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(native)
	var projected struct {
		Geodata struct {
			Cron   string `json:"cron"`
			Assets []struct {
				File string `json:"file"`
			} `json:"assets"`
		} `json:"geodata"`
	}
	if json.Unmarshal(raw, &projected) != nil || projected.Geodata.Cron != legacy.GeodataCron || len(projected.Geodata.Assets) != 1 || projected.Geodata.Assets[0].File != "geosite.dat" || hashes["geoip.dat"] != "" {
		t.Fatal("IP geodata escaped to agent or cron was changed")
	}
}

func TestNodeMultiplierRequiredAndExactPrecision(t *testing.T) {
	for _, value := range []string{"0", "1", "0.000001", "1.123456", "1000"} {
		var node Node
		if err := json.Unmarshal([]byte(`{"multiplier":`+value+`}`), &node); err != nil {
			t.Fatalf("valid multiplier %s: %v", value, err)
		}
	}
	for _, document := range []string{`{}`, `{"multiplier":null}`, `{"multiplier":-1}`, `{"multiplier":0.0000001}`, `{"multiplier":"1"}`} {
		var node Node
		if err := json.Unmarshal([]byte(document), &node); err == nil {
			t.Fatalf("invalid multiplier accepted: %s", document)
		}
	}
}

func TestBandwidthUsesNativeUserValidation(t *testing.T) {
	for _, policy := range []*BandwidthPolicy{nil, {NormalMbps: 100}, {NormalMbps: 100, BurstMbps: 200, AfterSeconds: 30, ResetSeconds: 60}} {
		if err := validateBandwidth(policy); err != nil {
			t.Fatal(err)
		}
	}
	for _, policy := range []*BandwidthPolicy{{NormalMbps: 0}, {NormalMbps: 100, BurstMbps: 50}, {NormalMbps: 100, AfterSeconds: 3601}} {
		if err := validateBandwidth(policy); err == nil {
			t.Fatal("invalid speed policy accepted")
		}
	}
}

func TestActivityRequiresNativeMeasuredSnapshot(t *testing.T) {
	now := time.Now()
	snapshot := liveSnapshot{receivedAt: now, hello: wire.AgentHello{Capabilities: []string{"sampled_activity_v1"}}, report: wire.TelemetryReport{Ready: true, Synchronized: true}}
	if !activityAvailable(snapshot, now, 5) {
		t.Fatal("current native activity unavailable")
	}
	if activityAvailable(snapshot, now.Add(time.Minute), 5) {
		t.Fatal("stale activity accepted")
	}
	snapshot.hello.Capabilities = nil
	if activityAvailable(snapshot, now, 5) {
		t.Fatal("old open-session counters presented as active")
	}
}

func TestSharedGeositeBlocksUseAnalysisOnlyAssetSubstitution(t *testing.T) {
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[],"outbounds":[{"tag":"direct","protocol":"freedom"}]}`), Users: []wire.User{}, BlockedDomains: []string{"geosite:google"}}
	result := ValidateCompiled(config)
	if !result.Valid || !result.NeedsAgentValidation || config.BlockedDomains[0] != "geosite:google" {
		t.Fatalf("shared geosite validation mutated desired policy or required local files: %+v", result)
	}
	config.BlockedDomains = []string{"regexp:["}
	if ValidateCompiled(config).Valid {
		t.Fatal("asset substitution hid an invalid expression")
	}
}

func TestDisablingRefreshRetainsTheLastResolvedCountryGeneration(t *testing.T) {
	service := NewService(nil, nil, nil, nil, nil)
	now := time.Now()
	service.now = func() time.Time { return now }
	config := DefaultGlobal()
	config.SourceGeoIP.Enabled = true
	config.GeoIPURL = "https://assets.example/geoip.dat"
	key, _ := json.Marshal(struct {
		URL, Hash string
		Selectors []string
	}{config.GeoIPURL, config.GeoIPHash, config.SourceGeoIP.Selectors})
	oldSchedule, _ := json.Marshal(struct {
		Hours *int
		Cron  string
	}{config.GeodataIntervalHours, ""})
	service.policyAssets.key = string(key)
	service.policyAssets.scheduleKey = string(oldSchedule)
	service.policyAssets.generation = "last-good"
	service.policyAssets.prefixes = []string{"1.1.1.0/24"}
	service.policyAssets.next = now.Add(-time.Minute)
	service.policyAssets.completedAt = now.Add(-25 * time.Hour)
	disabled := 0
	config.GeodataIntervalHours = &disabled
	policy, err := service.sourceGeoIP(context.Background(), config, wire.FirewallPolicy{Enabled: true}, json.RawMessage(`{"inbounds":[{"tag":"paid","protocol":"vless","port":443}]}`))
	if err != nil || policy.Generation != "last-good" || len(policy.Prefixes) != 1 || !service.policyAssets.next.IsZero() {
		t.Fatalf("disabling refresh performed external work or discarded the last valid generation: %+v %v", policy, err)
	}
}

func TestStateRequestsCoalesceWithoutBlockingTheTrafficReader(t *testing.T) {
	master := &Master{stateWake: make(chan struct{}, 1)}
	peer := &agentPeer{}
	for range 100 {
		if err := master.receive(context.Background(), peer, wire.New("state.request", "", 0, map[string]any{})); err != nil {
			t.Fatal(err)
		}
	}
	if !peer.forceSync || len(master.stateWake) != 1 {
		t.Fatal("state requests were lost or produced unbounded work")
	}
}
