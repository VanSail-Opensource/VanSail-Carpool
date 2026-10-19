package subscription

// Xray camel-case options map to the documented Mihomo/Stash XHTTP keys.
// Server-only admission/buffering settings never become client options.
func yamlXHTTPOptions(source map[string]any) map[string]any {
	out := map[string]any{}
	extra := object(source, "extra")
	for _, field := range []string{"path", "host", "mode", "headers"} {
		if value, ok := source[field]; ok {
			out[field] = value
		} else if value, ok := extra[field]; ok {
			out[field] = value
		}
	}
	fields := map[string]string{
		"noGRPCHeader": "no-grpc-header", "xPaddingBytes": "x-padding-bytes", "xPaddingObfsMode": "x-padding-obfs-mode",
		"xPaddingKey": "x-padding-key", "xPaddingHeader": "x-padding-header", "xPaddingPlacement": "x-padding-placement", "xPaddingMethod": "x-padding-method",
		"uplinkHTTPMethod": "uplink-http-method", "sessionPlacement": "session-placement", "sessionKey": "session-key", "sessionTable": "session-table", "sessionLength": "session-length",
		"seqPlacement": "seq-placement", "seqKey": "seq-key", "uplinkDataPlacement": "uplink-data-placement", "uplinkDataKey": "uplink-data-key", "uplinkChunkSize": "uplink-chunk-size",
		"scMaxEachPostBytes": "sc-max-each-post-bytes", "scMinPostsIntervalMs": "sc-min-posts-interval-ms",
	}
	for native, client := range fields {
		if value, ok := extra[native]; ok {
			out[client] = value
		} else if value, ok := source[native]; ok {
			out[client] = value
		}
	}
	if reuse := yamlXHTTPReuse(object(extra, "xmux")); len(reuse) > 0 {
		out["reuse-settings"] = reuse
	}
	if download := object(extra, "downloadSettings"); len(download) > 0 {
		out["download-settings"] = yamlXHTTPDownload(download)
	}
	return out
}

func yamlXHTTPReuse(source map[string]any) map[string]any {
	out := map[string]any{}
	for native, client := range map[string]string{"maxConcurrency": "max-concurrency", "maxConnections": "max-connections", "cMaxReuseTimes": "c-max-reuse-times", "hMaxRequestTimes": "h-max-request-times", "hMaxReusableSecs": "h-max-reusable-secs", "hKeepAlivePeriod": "h-keep-alive-period"} {
		if value, ok := source[native]; ok {
			out[client] = value
		}
	}
	return out
}

func yamlXHTTPDownload(source map[string]any) map[string]any {
	out := map[string]any{}
	for native, client := range map[string]string{"address": "server", "port": "port"} {
		if value, ok := source[native]; ok {
			out[client] = value
		}
	}
	settings := object(source, "xhttpSettings")
	for _, key := range []string{"path", "host", "headers"} {
		if value, ok := settings[key]; ok {
			out[key] = value
		}
	}
	if reuse := yamlXHTTPReuse(object(object(settings, "extra"), "xmux")); len(reuse) > 0 {
		out["reuse-settings"] = reuse
	}
	security := text(source, "security")
	if security == "none" {
		out["tls"] = false
	} else if security == "tls" || security == "reality" {
		out["tls"] = true
		tls := object(source, security+"Settings")
		for native, client := range map[string]string{"serverName": "sni", "fingerprint": "client-fingerprint", "alpn": "alpn", "allowInsecure": "skip-cert-verify"} {
			if value, ok := tls[native]; ok {
				out[client] = value
			}
		}
		if security == "reality" {
			out["reality-opts"] = map[string]any{"public-key": tls["publicKey"], "short-id": tls["shortId"], "support-x25519mlkem768": true}
		}
	}
	return out
}
