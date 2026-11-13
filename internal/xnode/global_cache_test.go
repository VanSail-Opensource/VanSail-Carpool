package xnode

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

type globalCacheSettings struct {
	Settings
	reads int
	fail  bool
}

func (s *globalCacheSettings) Optional(context.Context, string) (string, error) {
	s.reads++
	return "", nil
}
func (s *globalCacheSettings) PutXNodeGlobalAtVersion(context.Context, string, string, int64, time.Time) error {
	if s.fail {
		return errors.New("write failed")
	}
	return nil
}

func TestGlobalSnapshotPublishesOnlyCommittedSettings(t *testing.T) {
	settings := &globalCacheSettings{}
	s := NewService(nil, settings, nil, nil, nil)
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if _, err := s.Global(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if settings.reads != 1 {
		t.Fatal("settings miss was not coalesced", settings.reads)
	}
	next, _ := s.Global(context.Background())
	next.ConnectionLimit = 42
	settings.fail = true
	if err := s.SaveGlobal(context.Background(), "admin", next); err == nil {
		t.Fatal("failed write succeeded")
	}
	got, _ := s.Global(context.Background())
	if got.ConnectionLimit != 0 {
		t.Fatal("uncommitted admission setting was published", got.ConnectionLimit)
	}
	settings.fail = false
	if err := s.SaveGlobal(context.Background(), "admin", next); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Global(context.Background())
	if got.ConnectionLimit != 42 || settings.reads != 1 {
		t.Fatal("committed setting absent or reread", got.ConnectionLimit, settings.reads)
	}
}

func TestGlobalSnapshotDoesNotShareCredentialsOrPolicies(t *testing.T) {
	value := DefaultGlobal()
	value.Providers["dns"] = wire.Provider{Type: "cloudflare", Credentials: map[string]string{"token": "private"}}
	cache := &globalSettingsCache{}
	cache.publish(value)
	value.Providers["dns"].Credentials["token"] = "caller modified"
	value.SourceGeoIP.Selectors[0] = "geoip:us"
	first, _ := cache.read(context.Background(), nil)
	first.Providers["dns"].Credentials["token"] = "redacted"
	first.FirewallDetection.Rules["xray_tcp_probe"] = wire.FirewallRule{}
	*first.GeodataIntervalHours = 0
	first.SourceGeoIP.Selectors[0] = "geoip:gb"
	first.AutomaticWhitelist.DurationHours = 1
	second, _ := cache.read(context.Background(), nil)
	if second.Providers["dns"].Credentials["token"] != "private" || second.SourceGeoIP.Selectors[0] != "geoip:cn" || *second.GeodataIntervalHours != 24 || !second.FirewallDetection.Rules["xray_tcp_probe"].Enabled || second.AutomaticWhitelist.DurationHours != 24 {
		t.Fatal("caller mutation changed a private global snapshot")
	}
}
