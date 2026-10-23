package subscription

import (
	"encoding/json"
	"fmt"
	"strings"
)

func (p Proxy) applyTuning(format string, out map[string]any) (map[string]any, error) {
	if p.Tuning == nil && format == "xray" {
		return out, nil
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	values := map[string]any{}
	if p.Tuning != nil {
		values, err = p.Tuning.values()
	}
	if err != nil {
		return nil, err
	}
	for _, spec := range allTuningSpecs() {
		value, exists := values[spec.Key]
		path := spec.Mappings[format]
		if !exists || path == "" || tuningReason(p, spec, format, value, values) != "" {
			continue
		}
		if format == "xray" && strings.HasPrefix(path, "streamSettings.xhttpSettings.extra.") {
			preserveXHTTPExtra(result)
		}
		security := text(p.Stream, "security")
		path = strings.ReplaceAll(path, "$security", security+"Settings")
		if spec.Kind == "range" {
			value = wireTuningRange(value.(map[string]any))
		}
		if spec.Kind == "headers" {
			headers := map[string]any{}
			for _, raw := range value.([]any) {
				row := raw.(map[string]any)
				headers[row["name"].(string)] = row["value"]
			}
			value = headers
		}
		if (format == "clash" || format == "stash") && spec.Key == "ssNetwork" {
			value = value != "tcp"
		}
		if format == "sing-box" && spec.Key == "ssNetwork" && value == "tcp,udp" {
			delete(result, "network")
			continue
		}
		if format == "sing-box" && spec.Key == "fingerprint" {
			setTuningPath(result, "tls.utls.enabled", true)
		}
		if format == "sing-box" && spec.Key == "curvePreferences" {
			curves := []string{}
			for _, curve := range value.([]any) {
				curves = append(curves, strings.TrimPrefix(curve.(string), "Curve"))
			}
			value = curves
		}
		if spec.Kind == "headers" {
			mergeTuningHeaders(result, path, value.(map[string]any))
		} else {
			setTuningPath(result, path, value)
		}
	}
	applyClientPolicy(format, result)
	return result, nil
}

func wireTuningRange(value map[string]any) any {
	low, high := int64(value["min"].(float64)), int64(value["max"].(float64))
	if low == high {
		return low
	}
	return fmt.Sprintf("%d-%d", low, high)
}

func setTuningPath(root map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := root
	for _, key := range parts[:len(parts)-1] {
		child, _ := current[key].(map[string]any)
		if child == nil {
			child = map[string]any{}
			current[key] = child
		}
		current = child
	}
	current[parts[len(parts)-1]] = value
}

func mergeTuningHeaders(root map[string]any, path string, headers map[string]any) {
	current := root
	for _, key := range strings.Split(path, ".") {
		child, _ := current[key].(map[string]any)
		if child == nil {
			child = map[string]any{}
			current[key] = child
		}
		current = child
	}
	for name, value := range headers {
		for existing := range current {
			if strings.EqualFold(existing, name) {
				delete(current, existing)
			}
		}
		current[name] = value
	}
}
