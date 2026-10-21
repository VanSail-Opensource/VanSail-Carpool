package subscription

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func (p Proxy) applyTuning(format string, out map[string]any) (map[string]any, error) {
	if p.Tuning == nil {
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
	values, err := p.Tuning.values()
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
		if strings.Contains(path, "downloadSettings.") {
			prepareDownloadTuning(result)
		}
		security := text(p.Stream, "security")
		if strings.HasPrefix(spec.Context, "download:") {
			if down := object(object(object(p.Stream, "xhttpSettings"), "extra"), "downloadSettings"); len(down) > 0 {
				security = text(down, "security")
			}
		}
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
		if format == "xray" && (spec.Key == "cipherSuites" || spec.Key == "downloadCipherSuites") {
			names := []string{}
			for _, v := range value.([]any) {
				names = append(names, v.(string))
			}
			value = strings.Join(names, ":")
		}
		if format == "sing-box" && (spec.Key == "tcpKeepAliveInterval" || spec.Key == "tcpKeepAliveIdle" || spec.Key == "connectTimeoutMs") {
			n := value.(float64)
			if n < 0 {
				setTuningPath(result, "disable_tcp_keep_alive", true)
				continue
			}
			unit := "s"
			if spec.Key == "connectTimeoutMs" {
				unit = "ms"
			}
			value = strconv.FormatInt(int64(n), 10) + unit
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
		if format == "clash" && (spec.Key == "fingerprint" || spec.Key == "downloadFingerprint") && value == "ios" {
			value = "iOS"
		}
		if spec.Kind == "headers" {
			mergeTuningHeaders(result, path, value.(map[string]any))
		} else {
			setTuningPath(result, path, value)
		}
	}
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

func prepareDownloadTuning(root map[string]any) {
	stream := object(root, "streamSettings")
	if stream == nil {
		return
	}
	xhttp := object(stream, "xhttpSettings")
	extra := object(xhttp, "extra")
	if extra == nil {
		extra = map[string]any{}
		xhttp["extra"] = extra
	}
	if object(extra, "downloadSettings") != nil {
		return
	}
	// Inherit the administrator's transport/security identity. Only the member's
	// allowlisted download tuning is then applied; no address/key/path is editable.
	// root has already passed a JSON round trip, so this clone cannot contain
	// unsupported values. Copy recursively without an ignored marshal error.
	download := cloneTuningObject(stream)
	delete(object(object(download, "xhttpSettings"), "extra"), "downloadSettings")
	extra["downloadSettings"] = download
}
