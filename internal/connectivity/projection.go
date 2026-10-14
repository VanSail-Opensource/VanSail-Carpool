package connectivity

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/google/uuid"
)

// Projection follows Remnawave's resolvedProxyConfigs and native Xray JSON
// generator. Resolved documents and generated credentials remain process-local.
func projectTarget(target Target) ([]byte, error) {
	outbound, err := projectOutbound(target)
	if err != nil {
		return nil, err
	}
	config := map[string]any{
		"log":      map[string]any{"loglevel": "none", "access": "none", "error": "none"},
		"inbounds": []any{}, "outbounds": []any{outbound},
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, projectionError("INVALID_RESOLVED_CONFIG")
	}
	return data, nil
}

// ClientOutbound returns the checked native client projection, including owner
// credentials. Callers must keep it out of public caches, logs and audit payloads.
func ClientOutbound(target Target) (map[string]any, error) { return projectOutbound(target) }

func projectOutbound(target Target) (map[string]any, error) {
	if len(target.NativeOutbound) > 0 {
		if len(target.NativeOutbound) > 64<<10 {
			return nil, projectionError("INVALID_RESOLVED_CONFIG")
		}
		var outbound map[string]any
		if json.Unmarshal(target.NativeOutbound, &outbound) != nil || outbound == nil {
			return nil, projectionError("INVALID_RESOLVED_CONFIG")
		}
		outbound["tag"] = "connectivity-" + target.HostUUID
		if err := validateProjectedOutbound(outbound); err != nil {
			return nil, err
		}
		return outbound, nil
	}
	if len(target.Resolved) == 0 || len(target.Resolved) > 2<<20 {
		return nil, projectionError("INVALID_RESOLVED_CONFIG")
	}
	var host map[string]any
	decoder := json.NewDecoder(bytes.NewReader(target.Resolved))
	decoder.UseNumber()
	if err := decoder.Decode(&host); err != nil || host == nil {
		return nil, projectionError("INVALID_RESOLVED_CONFIG")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, projectionError("INVALID_RESOLVED_CONFIG")
	}
	if !boundedConfigDepth(host, 0) {
		return nil, projectionError("INVALID_RESOLVED_CONFIG")
	}
	metadata := configObject(host["metadata"])
	id, err := uuid.Parse(stringValue(metadata["uuid"]))
	requested, requestedErr := uuid.Parse(target.HostUUID)
	if err != nil || requestedErr != nil || id == uuid.Nil || id != requested {
		return nil, projectionError("HOST_IDENTITY_MISMATCH")
	}
	client := configObject(host["clientOverrides"])
	if template := client["xrayJsonTemplate"]; template != nil {
		return nil, projectionError("UNSUPPORTED_TEMPLATE")
	}
	settings, err := projectProtocol(host)
	if err != nil {
		return nil, err
	}
	stream, err := projectStream(host)
	if err != nil {
		return nil, err
	}
	outbound := map[string]any{
		"tag": "connectivity-" + id.String(), "protocol": host["protocol"],
		"settings": settings, "streamSettings": stream,
	}
	if mux := configObject(host["mux"]); len(mux) > 0 {
		outbound["mux"] = pickFields(mux, "enabled", "concurrency", "xudpConcurrency", "xudpProxyUDP443")
	}
	outbound, err = applyXrayMapper(outbound, host)
	if err != nil {
		return nil, err
	}
	// Display tags are mutable. Identity and the isolated route are owned here.
	outbound["tag"] = "connectivity-" + id.String()
	if err := validateProjectedOutbound(outbound); err != nil {
		return nil, err
	}
	return outbound, nil
}

func projectionError(code string) error {
	return &CodeError{Code: "CONNECTIVITY_" + code}
}

func configObject(value any) map[string]any {
	object, _ := value.(map[string]any)
	return object
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func pickFields(source map[string]any, fields ...string) map[string]any {
	result := make(map[string]any, len(fields))
	for _, field := range fields {
		if value, ok := source[field]; ok && value != nil {
			result[field] = value
		}
	}
	return result
}

func nonemptyString(value any) bool {
	return strings.TrimSpace(stringValue(value)) != ""
}

func boundedConfigDepth(value any, depth int) bool {
	if depth > 64 {
		return false
	}
	switch item := value.(type) {
	case map[string]any:
		for _, child := range item {
			if !boundedConfigDepth(child, depth+1) {
				return false
			}
		}
	case []any:
		for _, child := range item {
			if !boundedConfigDepth(child, depth+1) {
				return false
			}
		}
	}
	return true
}
