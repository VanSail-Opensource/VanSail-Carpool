package iplookup

import "strings"

func finishTrojanOutcome(out probeOutcome) probeOutcome {
	if out.result == "" {
		out.result = "not_detected"
	}
	if len(out.findings) > 0 {
		out.result = "detected"
	}
	return out
}

// recordProbeOutcome replaces replayed task evidence and gates Trojan suspicion per port.
func recordProbeOutcome(port *PortProbe, key string, out probeOutcome) {
	kept := make([]ProbeFinding, 0, len(port.Findings)+len(out.findings))
	for _, f := range port.Findings {
		if f.TaskKey != key {
			kept = append(kept, f)
		}
	}
	for _, f := range out.findings {
		f.TaskKey = key
		kept = append(kept, f)
	}
	port.Findings = kept
	positive := map[string]bool{}
	for _, task := range port.Tasks {
		if strings.HasPrefix(task.Key, "trojan_") && task.Result == "detected" {
			positive[task.Key] = true
		}
	}
	for i := range port.Findings {
		f := &port.Findings[i]
		if f.Protocol == "trojan" && strings.HasPrefix(f.TaskKey, "trojan_") {
			f.Confidence = "observed"
			if len(positive) >= 2 {
				f.Confidence = "suspected"
			}
		}
	}
}
