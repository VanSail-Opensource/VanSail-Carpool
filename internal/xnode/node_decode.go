package xnode

import (
	"bytes"
	"encoding/json"
)

// The deprecated multiplier input is accepted but never affects new accounting.
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
	node.Multiplier, node.MultiplierFP = 1, MultiplierScale
	return nil
}

// MarshalJSON keeps the legacy API property constant even for historical nodes.
func (node Node) MarshalJSON() ([]byte, error) {
	type plain Node
	node.Multiplier = 1
	return json.Marshal(plain(node))
}
