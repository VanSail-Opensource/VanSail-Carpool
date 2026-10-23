package subscription

// applyClientPolicy runs after administrator mappers and member overrides.
// XHTTP keeps its transport-level XMUX; a second Mux/SMUX layer is forbidden.
func applyClientPolicy(format string, out map[string]any) {
	switch format {
	case "xray":
		stream := object(out, "streamSettings")
		if stream == nil {
			stream = map[string]any{}
			out["streamSettings"] = stream
		}
		enforceClientStream(stream)
		if text(stream, "network") == "xhttp" {
			delete(out, "mux")
		}
	case "sing-box":
		out["tcp_multi_path"] = false
		if text(object(out, "transport"), "type") == "xhttp" {
			delete(out, "multiplex")
		}
	case "clash", "stash":
		out["mptcp"] = false
		if text(out, "network") == "xhttp" {
			delete(out, "smux")
			delete(out, "mux")
			enforceClientDownload(object(out, "xhttp-opts"))
		}
	}
}

func enforceClientStream(stream map[string]any) {
	setTuningPath(stream, "sockopt.tcpMptcp", false)
	// Bound traversal to the real two-leg transport document.
	xhttp := object(stream, "xhttpSettings")
	for _, settings := range []map[string]any{xhttp, object(xhttp, "extra")} {
		down := object(settings, "downloadSettings")
		if down != nil {
			setTuningPath(down, "sockopt.tcpMptcp", false)
			delete(down, "mux")
			delete(down, "smux")
		}
	}
}

func enforceClientDownload(options map[string]any) {
	if down := object(options, "download-settings"); down != nil {
		down["mptcp"] = false
		delete(down, "smux")
		delete(down, "mux")
	}
}
