package subscription

import (
	"errors"
	"strings"
)

var mapperHostPaths = []string{
	"address", "clientOverrides.serverDescription", "finalRemark", "metadata.inboundTag",
	"metadata.remark", "metadata.tags", "mux", "port", "protocol", "protocolOptions",
	"security", "securityOptions", "streamOverrides.finalMask", "streamOverrides.sockopt",
	"transport", "transportOptions",
}

// ApplyClientMapper shares the documented ordered host mapping semantics across
// owner-scoped subscription renderers.
func ApplyClientMapper(outbound, host map[string]any, format string) (map[string]any, error) {
	mapper := configObject(configObject(host["clientOverrides"])["mapper"])
	operationsValue := mapper[format]
	if operationsValue == nil {
		return outbound, nil
	}
	operations, ok := operationsValue.([]any)
	if !ok || len(operations) > 256 {
		return nil, mapperError()
	}
	result := cloneConfigValue(outbound).(map[string]any)
	for _, item := range operations {
		operation := configObject(item)
		to, err := mapperSegments(stringValue(operation["to"]))
		if err != nil {
			return nil, err
		}
		var value any
		switch stringValue(operation["op"]) {
		case "set":
			var exists bool
			value, exists = operation["value"]
			if !exists {
				return nil, mapperError()
			}
		case "unset":
			mapperUpdate(result, to, nil, true)
			continue
		case "copy":
			var exists bool
			value, exists, err = mapperCopy(host, stringValue(operation["from"]))
			if err != nil {
				return nil, err
			}
			if !exists {
				continue
			}
		default:
			return nil, mapperError()
		}
		mapperUpdate(result, to, cloneConfigValue(value), false)
	}
	return result, nil
}

func mapperCopy(host map[string]any, from string) (any, bool, error) {
	var source any = configObject(host["metadata"])["rawInbound"]
	path := from
	if strings.HasPrefix(from, "$host.") {
		path = strings.TrimPrefix(from, "$host.")
		allowed := false
		for _, prefix := range mapperHostPaths {
			if path == prefix || strings.HasPrefix(path, prefix+".") || strings.HasPrefix(path, prefix+"[") {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, false, nil
		}
		source = host
	}
	segments, err := mapperSegments(path)
	if err != nil {
		return nil, false, err
	}
	value, found := mapperRead(source, segments)
	return value, found, nil
}

func cloneConfigValue(value any) any {
	switch item := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(item))
		for key, child := range item {
			result[key] = cloneConfigValue(child)
		}
		return result
	case []any:
		result := make([]any, len(item))
		for index, child := range item {
			result[index] = cloneConfigValue(child)
		}
		return result
	default:
		return value
	}
}

var errInvalidMapper = errors.New("invalid client mapper")

func mapperError() error { return errInvalidMapper }

func configObject(value any) map[string]any { object, _ := value.(map[string]any); return object }
func stringValue(value any) string          { text, _ := value.(string); return text }
