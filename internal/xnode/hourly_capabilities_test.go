package xnode

import (
	"encoding/json"
	"errors"
	"testing"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func TestHourlyNativeStateRequiresBothAdmissionAndHistoricalStreamCapabilities(t *testing.T) {
	config := wire.Config{Xray: json.RawMessage(`{"inbounds":[]}`), Users: []wire.User{{Hourly: &wire.HourlyUser{}}}}
	base := []string{"traffic_periods_v1", "traffic_period_barriers_v1", "volatile_connection_controls_v1", "measured_user_rates", "authorization_expiry", "member_outbound_guards"}
	for _, capability := range []string{wire.HourlyCapability, wire.TrafficStreamCapability} {
		if err := configurationCapabilities(config, append(base, capability)); !errors.Is(err, ErrUnavailable) {
			t.Fatal("one capability inferred the other", capability, err)
		}
	}
	if err := configurationCapabilities(config, append(base, wire.HourlyCapability, wire.TrafficStreamCapability)); err != nil {
		t.Fatal("complete native hourly capability set rejected", err)
	}
}
