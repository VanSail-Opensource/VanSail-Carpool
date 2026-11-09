package subscription

func ensureEndpointDownload(root map[string]any, p Proxy, values map[string]any) {
	stream := object(root, "streamSettings")
	parent := object(stream, "xhttpSettings")
	extra := effectiveXHTTP(parent)
	download := object(extra, "downloadSettings")
	if download == nil {
		download = map[string]any{}
		extra["downloadSettings"] = download
	}
	if download["network"] == nil {
		download["network"] = "xhttp"
	}
	if text(download, "address") == "" {
		download["address"] = p.Address
	}
	if download["port"] == nil || tuningEndpointMaximum(download["port"]) == 0 {
		download["port"] = p.Port
	}
	if download["security"] == nil {
		security := text(p.Stream, "security")
		if selected, ok := values["downloadSecurity"].(string); ok {
			security = selected
		}
		download["security"] = security
	}
	security := text(download, "security")
	securityKey := security + "Settings"
	settings := cloneTuningObject(object(stream, securityKey))
	mergeEndpointObject(settings, object(download, securityKey))
	if len(settings) > 0 {
		download[securityKey] = settings
	}
	// Pinned Xray does not inherit destination/path/session fields at dial time.
	// Materialize defaults while retaining each explicitly provided child.
	settings = cloneTuningObject(parent)
	delete(settings, "downloadSettings")
	delete(object(settings, "extra"), "downloadSettings")
	InheritXHTTPExtra(settings)
	provided := object(download, "xhttpSettings")
	for _, key := range []string{"path", "host", "mode"} {
		if value, ok := provided[key]; ok {
			settings[key] = value
		}
	}
	providedExtra := cloneTuningObject(effectiveXHTTP(provided))
	for _, key := range []string{"path", "host", "mode", "extra", "downloadSettings"} {
		delete(providedExtra, key)
	}
	mergeEndpointXMUXChoice(object(object(settings, "extra"), "xmux"), object(providedExtra, "xmux"))
	mergeEndpointObject(object(settings, "extra"), providedExtra)
	download["xhttpSettings"] = settings
}

func mergeEndpointObject(target, source map[string]any) {
	for key, value := range source {
		if child, ok := value.(map[string]any); ok {
			destination := object(target, key)
			if destination == nil {
				destination = map[string]any{}
				target[key] = destination
			}
			mergeEndpointObject(destination, child)
		} else {
			target[key] = value
		}
	}
}
