package xnode

func validateMemberStream(stream map[string]any, names map[string]string) error {
	if !allowedFields(stream, "network", "security", "tlsSettings", "realitySettings", "tcpSettings", "wsSettings", "grpcSettings", "httpupgradeSettings", "xhttpSettings", "hysteriaSettings", "sockopt") {
		return ErrInvalid
	}
	network, _ := stream["network"].(string)
	if network != "" && network != "tcp" && network != "raw" && network != "ws" && network != "grpc" && network != "httpupgrade" && network != "xhttp" && network != "hysteria" {
		return ErrInvalid
	}
	security, _ := stream["security"].(string)
	if security != "" && security != "none" && security != "tls" && security != "reality" {
		return ErrInvalid
	}
	fields := map[string][]string{
		"tlsSettings":     {"serverName", "alpn", "fingerprint", "allowInsecure", "pinnedPeerCertSha256", "verifyPeerCertByName"},
		"realitySettings": {"serverName", "publicKey", "password", "shortId", "fingerprint", "spiderX", "mldsa65Verify"},
		"wsSettings":      {"path", "host", "headers", "heartbeatPeriod"}, "grpcSettings": {"serviceName", "authority", "multiMode"},
		"httpupgradeSettings": {"path", "host", "headers"}, "xhttpSettings": {"path", "host", "mode"}, "tcpSettings": {},
		"hysteriaSettings": {"version", "auth"}, "sockopt": {"dialerProxy"},
	}
	for field, keys := range fields {
		value, exists := stream[field]
		if !exists {
			continue
		}
		object, ok := value.(map[string]any)
		if !ok || !allowedFields(object, keys...) {
			return ErrInvalid
		}
	}
	if socket, _ := stream["sockopt"].(map[string]any); socket != nil {
		tag, _ := socket["dialerProxy"].(string)
		if names[tag] == "" {
			return ErrInvalid
		}
	}
	return nil
}
