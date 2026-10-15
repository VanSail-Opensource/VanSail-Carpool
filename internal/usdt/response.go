package usdt

import (
	"bytes"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/jsonlimit"
	"io"
)

// responseValue decodes an envelope field directly into its typed destination.
// The decoder's input is borrowed only during UnmarshalJSON, never retained.
type responseValue struct {
	target any
	valid  bool
}

func (v *responseValue) UnmarshalJSON(raw []byte) error {
	v.valid = false
	if bytes.Equal(raw, []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(raw, v.target); err != nil {
		return err
	}
	v.valid = true
	return nil
}

type typedRPCReply struct {
	ID     uint64        `json:"id"`
	Result responseValue `json:"result"`
	Error  *struct {
		Code int `json:"code"`
	} `json:"error"`
}

// decodeResponse bounds the complete document, including trailing whitespace,
// and rejects extra documents just as json.Unmarshal did.
func decodeResponse(reader io.Reader, output any) error {
	limited := &io.LimitedReader{R: reader, N: maxRPCResponseBytes + 1}
	decoder := json.NewDecoder(jsonlimit.NewReader(limited, rpcJSONLimits()))
	if err := decoder.Decode(output); err != nil {
		return ErrRPC
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF || limited.N == 0 {
		return ErrRPC
	}
	return nil
}

func rpcJSONLimits() jsonlimit.Limits {
	return jsonlimit.Limits{Depth: 64, ArrayItems: 8192, Nodes: 65536, StringBytes: maxRPCResponseBytes}
}
