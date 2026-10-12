package iplookup

// sectionMeta is satisfied by every section type that embeds LiveMeta, so task
// projection can read a finished section's availability without owning
// per-section parsing.
type sectionMeta interface{ sectionMeta() LiveMeta }

func (m LiveMeta) sectionMeta() LiveMeta { return m }

// liveTaskStatus derives one finished phase's task verdict from its section
// value. Observed emptiness and unconfigured credentials are valid outcomes:
// empty counts as success, unconfigured as skipped.
func liveTaskStatus(phase string, value any) ProgressTask {
	task := ProgressTask{Key: phase, Status: "success"}
	switch v := value.(type) {
	case BGPTopology:
		switch phase {
		case "relationships":
			applyMeta(&task, v.RelationshipMeta)
		case "topology":
			applyMeta(&task, v.LiveMeta)
		}
		// Later graph rewrites (names) never own the topology-level meta.
	case []ASNDetails:
		applyASNList(&task, v)
	default:
		if carrier, ok := value.(sectionMeta); ok {
			applyMeta(&task, carrier.sectionMeta())
		}
	}
	return task
}

func applyMeta(task *ProgressTask, meta LiveMeta) {
	switch meta.Status {
	case "error":
		task.Status, task.ErrorCode = "error", meta.ErrorCode
	case "partial":
		task.Status = "partial"
	case "unconfigured", "skipped":
		task.Status = "skipped"
	}
}

// applyASNList marks the asns task error only when every origin lost all of
// its sources, and partial when any origin lost at least one source.
func applyASNList(task *ProgressTask, asns []ASNDetails) {
	var failed, degraded int
	for _, asn := range asns {
		if len(asn.Sources) == 0 {
			continue
		}
		errored := 0
		for _, meta := range asn.Sources {
			if meta.Status == "error" {
				errored++
			}
		}
		switch {
		case errored == len(asn.Sources):
			failed++
		case errored > 0:
			degraded++
		}
	}
	switch {
	case len(asns) > 0 && failed == len(asns):
		task.Status = "error"
	case failed > 0 || degraded > 0:
		task.Status = "partial"
	}
}
