package xnode

import (
	"bytes"
	"encoding/json"
	"regexp"
)

var multiplierJSON = regexp.MustCompile(`^(0|[1-9][0-9]{0,3})(\.[0-9]{1,6})?$`)

// UnmarshalJSON retains the required multiplier distinction between omitted,
// null, explicit zero and a value outside the fixed-point precision.
func (node *Node) UnmarshalJSON(raw []byte) error {
	type plain Node
	var input struct {
		*plain
		Multiplier json.RawMessage `json:"multiplier"`
	}
	input.plain = (*plain)(node)
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return err
	}
	if !multiplierJSON.Match(bytes.TrimSpace(input.Multiplier)) || json.Unmarshal(input.Multiplier, &node.Multiplier) != nil {
		return ErrInvalid
	}
	return nil
}
