package httpapi

import (
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestDashboardResponseRetainsBenefitLabelsAndReadiness(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  model.Dashboard
		labels string
		ready  string
	}{
		{"active", model.Dashboard{ActiveSquadNames: []string{"Paid hidden squad", "Second"}, SubscriptionReady: true}, `["Paid hidden squad","Second"]`, "true"},
		{"empty", model.Dashboard{}, "[]", "false"},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := json.Marshal(mapDashboardResponse(test.value))
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["activeSquadNames"]) != test.labels || string(fields["subscriptionReady"]) != test.ready {
				t.Fatalf("dashboard omitted domain projection: %s", raw)
			}
		})
	}
}
