package xnode

import (
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

// Retain intent on every outcome and only the documented transaction fields.
// New native output excludes utility paths; existing receipt facts are retained.
func updateOperationResult(operation providerops.Operation, request CommandRequest, node string, result json.RawMessage) (string, error) {
	facts := map[string]any{"command": request.Name, "nodeId": node}
	var previous map[string]json.RawMessage
	if operation.ResultJSON != "" && json.Unmarshal([]byte(operation.ResultJSON), &previous) != nil {
		return "", ErrUpdateEvidence
	}
	if expected, ok := UpdateExpectationFromResult(operation.ResultJSON); ok {
		facts["updateExpectation"] = expected
	} else if retained := previous["previousResult"]; len(retained) > 0 {
		facts["previousResult"] = retained
	} else if len(previous) > 0 {
		facts["previousResult"] = previous
	}
	var priorTransaction UpdateTransaction
	if json.Unmarshal(previous["updateTransaction"], &priorTransaction) == nil && priorTransaction.ID != "" && priorTransaction.bounded() {
		facts["updateTransaction"] = priorTransaction
	}
	var transaction UpdateTransaction
	if json.Unmarshal(result, &transaction) == nil && transaction.ID != "" && transaction.bounded() {
		facts["updateTransaction"] = transaction
	}
	var unchanged struct {
		Unchanged bool   `json:"unchanged"`
		Current   string `json:"current"`
	}
	if json.Unmarshal(result, &unchanged) == nil && unchanged.Unchanged && len(unchanged.Current) <= 128 {
		facts["unchanged"], facts["runningVersion"] = true, unchanged.Current
	}
	raw, err := json.Marshal(facts)
	return string(raw), err
}
