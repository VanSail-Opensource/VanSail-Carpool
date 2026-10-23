package subscription

import (
	"fmt"
	"slices"

	"strings"

	"golang.org/x/net/http/httpguts"
)

func ValidateTuning(overrides ConnectionOverrides) map[string]string {
	overrides = ActiveTuning(overrides)
	errors := map[string]string{}
	values, err := overrides.values()
	if err != nil {
		errors["overrides"] = "invalid"
		return errors
	}
	for _, spec := range allTuningSpecs() {
		value, exists := values[spec.Key]
		if !exists {
			continue
		}
		invalid := false
		switch spec.Kind {
		case "integer", "enum-number":
			n, ok := value.(float64)
			invalid = !ok || n < float64(spec.Minimum) || n > float64(spec.Maximum)
			if spec.Kind == "enum-number" && !slices.Contains(spec.Enum, fmt.Sprint(n)) {
				invalid = true
			}
		case "enum":
			invalid = !slices.Contains(spec.Enum, fmt.Sprint(value))
		case "string":
			invalid = len(fmt.Sprint(value)) > int(spec.Maximum) || strings.ContainsAny(fmt.Sprint(value), "\x00\r\n")
		case "range":
			r := value.(map[string]any)
			low, high := r["min"].(float64), r["max"].(float64)
			invalid = low < 0 || high > 2147483647 || low > high
			if strings.HasSuffix(spec.Key, "XPaddingBytes") && low == 0 && high != 0 {
				invalid = true
			}
		case "strings":
			seen := map[string]bool{}
			for _, raw := range value.([]any) {
				text := raw.(string)
				if len(text) > 128 || seen[text] || len(spec.Enum) > 0 && !slices.Contains(spec.Enum, text) {
					invalid = true
				}
				seen[text] = true
			}
			invalid = invalid || len(value.([]any)) > 64
		case "headers":
			invalid = !validTuningHeaders(value.([]any))
		}
		if invalid {
			errors[spec.Key] = "invalid"
		}
	}
	if values["tcpMptcp"] == true {
		errors["tcpMptcp"] = "fixedOff"
	}
	if tuningRangePositive(values["xmuxMaxConcurrency"]) && tuningRangePositive(values["xmuxMaxConnections"]) {
		errors["xmuxMaxConnections"] = "conflict"
	}
	for _, prefix := range []string{"singboxMux", "mihomoMux"} {
		if positiveNumber(values[prefix+"MaxStreams"]) && (positiveNumber(values[prefix+"MaxConnections"]) || positiveNumber(values[prefix+"MinStreams"])) {
			errors[prefix+"MaxStreams"] = "conflict"
		}
	}
	if _, ok := values["ssUDPOverTCPVersion"]; ok && values["ssUDPOverTCP"] != true {
		errors["ssUDPOverTCPVersion"] = "dependency"
	}
	for _, key := range []string{"ssUDPOverTCP", "singboxMuxEnabled", "mihomoMuxEnabled"} {
		if values[key] == true {
			errors[key] = "serverCapability"
		}
	}
	return errors
}

func positiveNumber(value any) bool { n, _ := value.(float64); return n > 0 }
func tuningRangePositive(value any) bool {
	v, _ := value.(map[string]any)
	return positiveNumber(v["max"])
}

func validTuningHeaders(values []any) bool {
	if len(values) > 32 {
		return false
	}
	seen := map[string]bool{}
	for _, raw := range values {
		row := raw.(map[string]any)
		name, value := row["name"].(string), row["value"].(string)
		key := strings.ToLower(name)
		if !httpguts.ValidHeaderFieldName(name) || !httpguts.ValidHeaderFieldValue(value) || len(name) > 128 || len(value) > 4096 || seen[key] {
			return false
		}
		if slices.Contains([]string{"host", "authorization", "proxy-authorization", "cookie", "content-length", "transfer-encoding", "connection", "upgrade", "content-type"}, key) {
			return false
		}
		seen[key] = true
	}
	return true
}
