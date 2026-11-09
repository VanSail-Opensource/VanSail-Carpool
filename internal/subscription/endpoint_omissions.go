package subscription

// Optional omissions must remove inherited/mapped values as well as skip new
// assignments, otherwise a diagnostic could claim a field was omitted while it
// still leaked from the base client projection.
func pruneEndpointOmissions(p Proxy, format string, out map[string]any) {
	values := endpointValues(p)
	for _, spec := range allEndpointSpecs() {
		value, present := values[spec.Key]
		if !present {
			value, present = endpointInheritedValue(p, spec)
		}
		path := spec.Mappings[format]
		reason := endpointOptionReason(p, spec, format, value, values)
		if !present || path == "" || reason == "" || reason == "context" || reason == "dependency" || reason == "serverMode" || reason == "fixedOff" || reason == "conflict" || spec.Key == "tcpMptcp" {
			continue
		}
		path = endpointXrayPath(path, p, values)
		deleteTuningPath(out, path)
	}
}
