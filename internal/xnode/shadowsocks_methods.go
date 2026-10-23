package xnode

// validShadowsocksMethods limits managed listener compilation to native methods.
func validShadowsocksMethods(native map[string]any) bool {
	inbounds, _ := native["inbounds"].([]any)
	for _, raw := range inbounds {
		inbound, _ := raw.(map[string]any)
		if inbound["protocol"] != "shadowsocks" {
			continue
		}
		settings, _ := inbound["settings"].(map[string]any)
		method := settings["method"]
		if method != "chacha20-ietf-poly1305" && method != "2022-blake3-aes-128-gcm" && method != "2022-blake3-aes-256-gcm" {
			return false
		}
	}
	return true
}
