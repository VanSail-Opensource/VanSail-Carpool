package subscription

import (
	"fmt"
	"strconv"
	"strings"
)

func tuningEndpointMaximum(value any) float64 {
	parts := strings.Split(fmt.Sprint(value), "-")
	number, _ := strconv.ParseFloat(parts[len(parts)-1], 64)
	return number
}

func cloneTuningObject(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, child := range value {
		if object, ok := child.(map[string]any); ok {
			result[key] = cloneTuningObject(object)
		} else {
			result[key] = child
		}
	}
	return result
}

// Xray replaces its whole extra object when present. Copy inherited client
// fields into it before adding an override so unrelated administrator settings
// retain their behavior. Identity/transport keys stay at their original level.
func preserveXHTTPExtra(root map[string]any) {
	xhttp := object(object(root, "streamSettings"), "xhttpSettings")
	if xhttp == nil {
		return
	}
	extra := object(xhttp, "extra")
	if extra == nil {
		extra = map[string]any{}
		xhttp["extra"] = extra
	}
	for key, value := range xhttp {
		if key == "extra" || key == "mode" || key == "host" || key == "path" {
			continue
		}
		if _, exists := extra[key]; !exists {
			extra[key] = value
		}
	}
}
