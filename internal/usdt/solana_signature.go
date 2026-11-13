package usdt

import "encoding/json"

// The RPC transaction error is null, a unit-variant string, or an externally
// tagged variant object. Other JSON values are missing proof, not failure.
func solanaExecutionFailed(raw json.RawMessage) (bool, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return false, ErrProof
	}
	switch value := value.(type) {
	case nil:
		return false, nil
	case string:
		if value != "" {
			return true, nil
		}
	case map[string]any:
		if len(value) == 1 {
			for variant := range value {
				if variant != "" {
					return true, nil
				}
			}
		}
	}
	return false, ErrProof
}
