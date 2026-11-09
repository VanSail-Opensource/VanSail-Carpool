package subscription

import "strings"

func xmuxMaximum(value any) float64 {
	if interval, ok := value.(map[string]any); ok {
		return tuningEndpointMaximum(interval["max"])
	}
	return tuningEndpointMaximum(value)
}

func xmuxOpposite(key string) string {
	switch key {
	case "xmuxMaxConcurrency":
		return "xmuxMaxConnections"
	case "xmuxMaxConnections":
		return "xmuxMaxConcurrency"
	case "downloadXmuxMaxConcurrency":
		return "downloadXmuxMaxConnections"
	case "downloadXmuxMaxConnections":
		return "downloadXmuxMaxConcurrency"
	}
	return ""
}

func xmuxExplicitConflict(key string, values map[string]any) bool {
	other := xmuxOpposite(key)
	return other != "" && xmuxMaximum(values[key]) > 0 && xmuxMaximum(values[other]) > 0
}

// A single explicit positive choice switches only the opposing inherited mode.
// An explicit opposing zero remains explicit; two positive choices are rejected.
func applyXMUXChoice(root map[string]any, path, key string, value any, values map[string]any) {
	other := xmuxOpposite(key)
	if other == "" || xmuxMaximum(value) <= 0 {
		return
	}
	if _, explicit := values[other]; explicit {
		return
	}
	otherKey := "maxConcurrency"
	if strings.HasSuffix(key, "MaxConcurrency") {
		otherKey = "maxConnections"
	}
	if strings.HasSuffix(path, "max-concurrency") || strings.HasSuffix(path, "max-connections") {
		otherKey = strings.ReplaceAll(strings.ReplaceAll(otherKey, "Concurrency", "-concurrency"), "Connections", "-connections")
	}
	if index := strings.LastIndex(path, "."); index >= 0 {
		deleteTuningPath(root, path[:index+1]+otherKey)
	}
}

func mergeEndpointXMUXChoice(inherited, provided map[string]any) {
	for _, choice := range []struct{ key, other string }{{"maxConcurrency", "maxConnections"}, {"maxConnections", "maxConcurrency"}} {
		if _, explicitOther := provided[choice.other]; !explicitOther && xmuxMaximum(provided[choice.key]) > 0 {
			delete(inherited, choice.other)
		}
	}
}
