package app

import (
	"encoding/json"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
)

func TestConnectivityTargetsIncludesHiddenAndSkipsDisabled(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"finalRemark":"Hidden fixture","address":"proxy.example","port":443,"metadata":{"uuid":"99999999-9999-4999-8999-999999999999","isHidden":true,"isDisabled":false},"protocolOptions":{"id":"fixture-secret"}}`),
		json.RawMessage(`{"metadata":{"uuid":"88888888-8888-4888-8888-888888888888","isDisabled":true}}`),
	}
	targets, err := connectivityTargets(items)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0].Remark != "Hidden fixture" || len(targets[0].Resolved) == 0 {
		t.Fatal("accessible hidden host was not preserved")
	}
	encoded, err := json.Marshal(targets)
	if err != nil {
		t.Fatal(err)
	}
	var safe []map[string]any
	if err := json.Unmarshal(encoded, &safe); err != nil {
		t.Fatal(err)
	}
	if _, exists := safe[0]["Resolved"]; exists {
		t.Fatal("credentials escaped the source boundary")
	}
	if _, exists := safe[0]["protocolOptions"]; exists {
		t.Fatal("credentials escaped the source boundary")
	}
	if _, err := connectivityTargets(append(items, items[0])); connectivity.ErrorCode(err) != "CONNECTIVITY_INVALID_SUBSCRIPTION" {
		t.Fatal("ambiguous host identity was accepted")
	}
}
