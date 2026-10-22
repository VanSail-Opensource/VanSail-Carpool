package subscription

import (
	"fmt"
	"slices"
	"strings"
)

// SplitModes requires two administrator hosts reaching the same XHTTP listener.
// A node ID alone is insufficient: sessions belong to one inbound's handler.
func SplitModes(up, down Proxy) []string {
	if up.ID == down.ID || up.NodeID == "" || up.NodeID != down.NodeID || up.InboundTag == "" || up.InboundTag != down.InboundTag || up.Protocol != down.Protocol ||
		text(up.Stream, "network") != "xhttp" || text(down.Stream, "network") != "xhttp" {
		return nil
	}
	a, b := object(up.Stream, "xhttpSettings"), object(down.Stream, "xhttpSettings")
	if strings.Trim(text(a, "path"), "/") != strings.Trim(text(b, "path"), "/") {
		return nil
	}
	// Custom session placement must agree, as well as the listener/path.
	for _, key := range []string{"sessionPlacement", "sessionKey", "sessionTable", "sessionLength"} {
		if fmt.Sprint(xhttpOption(a, key)) != fmt.Sprint(xhttpOption(b, key)) {
			return nil
		}
	}
	mode := text(a, "mode")
	if mode == "stream-one" || text(b, "mode") == "stream-one" {
		return nil
	}
	if mode == "packet-up" {
		return []string{"packet-up"}
	}
	if mode == "stream-up" {
		return []string{"stream-up"}
	}
	return []string{"auto", "packet-up", "stream-up"}
}

func xhttpOption(settings map[string]any, key string) any {
	if extra := object(settings, "extra"); extra != nil {
		return extra[key]
	}
	return settings[key]
}

// SplitProxy derives both legs from current owner-scoped hosts. Credentials are
// retained only in this ephemeral projection, never in the saved reference.
func SplitProxy(id, mode string, up, down Proxy) (Proxy, error) {
	if !slices.Contains(SplitModes(up, down), mode) {
		return Proxy{}, ErrUnsupported
	}
	// Render administrator mappings first, then apply member tuning against the
	// combined stream so download options target the selected download host.
	options := ConnectionOverrides{}
	if up.Tuning != nil {
		options = *up.Tuning
	}
	options.XhttpMode = &mode
	up.Tuning, down.Tuning = nil, nil
	u, err := up.Outbound()
	if err != nil {
		return Proxy{}, err
	}
	d, err := down.Outbound()
	if err != nil {
		return Proxy{}, err
	}
	projectedUp, projectedDown := up, down
	projectedUp.Stream, projectedDown.Stream = object(u, "streamSettings"), object(d, "streamSettings")
	if u["protocol"] != up.Protocol || d["protocol"] != down.Protocol || !slices.Contains(SplitModes(projectedUp, projectedDown), mode) {
		return Proxy{}, ErrUnsupported
	}
	stream := cloneTuningObject(object(u, "streamSettings"))
	wrapped := map[string]any{"streamSettings": stream}
	preserveXHTTPExtra(wrapped)
	settings := object(stream, "xhttpSettings")
	settings["mode"] = mode
	download := cloneTuningObject(object(d, "streamSettings"))
	delete(object(object(download, "xhttpSettings"), "extra"), "downloadSettings")
	download["address"], download["port"] = down.Address, down.Port
	object(settings, "extra")["downloadSettings"] = download
	result := up
	result.ID = "split:" + id
	result.Name = up.Name + " ↑ / " + down.Name + " ↓"
	result.Stream = stream
	result.Tuning = &options
	result.MapperHost = nil
	result.ExcludedFormats = slices.Clone(up.ExcludedFormats)
	for _, format := range down.ExcludedFormats {
		if !slices.Contains(result.ExcludedFormats, format) {
			result.ExcludedFormats = append(result.ExcludedFormats, format)
		}
	}
	return result, nil
}
