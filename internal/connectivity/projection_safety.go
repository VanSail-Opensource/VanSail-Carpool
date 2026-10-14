package connectivity

import "strings"

// Mapper output is revalidated because upstream operations can replace complete
// settings or protocols. A missing proxy must never become a direct connection.
func validateProjectedOutbound(outbound map[string]any) error {
	for key := range outbound {
		switch key {
		case "tag", "protocol", "settings", "streamSettings", "mux", "sendThrough", "targetStrategy":
		default:
			return projectionError("UNSUPPORTED_CHAIN")
		}
	}
	if err := validateNoChains(outbound, 0); err != nil {
		return err
	}
	settings := configObject(outbound["settings"])
	var endpoint map[string]any
	switch stringValue(outbound["protocol"]) {
	case "vless", "vmess":
		endpoint = singleConfigObject(settings["vnext"])
		user := singleConfigObject(endpoint["users"])
		if !nonemptyString(user["id"]) {
			return projectionError("INVALID_RESOLVED_CONFIG")
		}
	case "trojan", "shadowsocks":
		endpoint = singleConfigObject(settings["servers"])
		if !nonemptyString(endpoint["password"]) {
			return projectionError("INVALID_RESOLVED_CONFIG")
		}
		if outbound["protocol"] == "shadowsocks" && !nonemptyString(endpoint["method"]) {
			return projectionError("INVALID_RESOLVED_CONFIG")
		}
	case "socks", "http":
		endpoint = singleConfigObject(settings["servers"])
	case "hysteria":
		endpoint = settings
		if numberValue(settings["version"]) != 2 {
			return projectionError("UNSUPPORTED_PROTOCOL")
		}
	default:
		return projectionError("UNSUPPORTED_PROTOCOL")
	}
	if !nonemptyString(endpoint["address"]) || !validProxyPort(endpoint["port"]) {
		return projectionError("INVALID_RESOLVED_CONFIG")
	}
	stream := configObject(outbound["streamSettings"])
	switch stringValue(stream["network"]) {
	case "", "raw", "tcp", "ws", "grpc", "xhttp", "httpupgrade", "kcp", "hysteria":
	default:
		return projectionError("UNSUPPORTED_TRANSPORT")
	}
	switch stringValue(stream["security"]) {
	case "", "none", "tls", "reality":
	default:
		return projectionError("UNSUPPORTED_SECURITY")
	}
	if outbound["protocol"] == "hysteria" && stream["network"] != "hysteria" {
		return projectionError("UNSUPPORTED_TRANSPORT")
	}
	if stream["network"] == "hysteria" {
		transport := configObject(stream["hysteriaSettings"])
		if numberValue(transport["version"]) != 2 || !nonemptyString(transport["auth"]) {
			return projectionError("INVALID_RESOLVED_CONFIG")
		}
	}
	return nil
}

func singleConfigObject(value any) map[string]any {
	items, ok := value.([]any)
	if !ok || len(items) != 1 {
		return nil
	}
	return configObject(items[0])
}

func validateNoChains(value any, depth int) error {
	if depth > 64 {
		return projectionError("INVALID_RESOLVED_CONFIG")
	}
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			name := strings.ToLower(key)
			if name == "headers" {
				continue // HTTP header names are data, not Xray configuration keys.
			}
			if (name == "dialerproxy" && child != nil && child != "") || name == "proxysettings" || name == "reverse" {
				return projectionError("UNSUPPORTED_CHAIN")
			}
			// Log/key files and local certificate files are outside an in-memory probe.
			if name == "masterkeylog" || name == "keylog" || name == "certificatefile" || name == "keyfile" {
				return projectionError("UNSUPPORTED_TEMPLATE")
			}
			if err := validateNoChains(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range item {
			if err := validateNoChains(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
