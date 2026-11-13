package iplookup

import (
	"context"
	"encoding/json"
)

// LiveOptions controls completion-cache reuse and request-local event delivery.
type LiveOptions struct {
	Refresh   bool
	CacheOnly bool
	Emit      func(LiveEvent) error
	runKey    string
	onAttempt func(context.Context) error
}

// LiveEvent carries immutable snapshots; section events replace one projection.
type LiveEvent struct {
	Type      string          `json:"type"`
	IP        string          `json:"ip"`
	Progress  *Progress       `json:"progress,omitempty"`
	Section   string          `json:"section,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	ErrorCode string          `json:"errorCode,omitempty"`
}

var livePhases = []string{"topology", "relationships", "names", "asns", "scores", "ptr", "shodan", "transfers"}

func liveComplete(details LiveDetails, data []byte) LiveEvent {
	progress := &Progress{Phase: "complete", Completed: len(livePhases), Total: len(livePhases), Tasks: make([]ProgressTask, 0, len(livePhases))}
	sections := map[string]any{
		"topology": details.Topology, "relationships": details.Topology, "names": details.Topology,
		"asns": details.ASNs, "scores": details.Scores, "ptr": details.PTR, "shodan": details.Shodan,
	}
	if details.Transfers != nil {
		sections["transfers"] = *details.Transfers
	}
	for _, phase := range livePhases {
		task := ProgressTask{Key: phase, Status: "skipped"}
		if section, ok := sections[phase]; ok {
			task = liveTaskStatus(phase, section)
		}
		progress.Tasks = append(progress.Tasks, task)
	}
	return LiveEvent{Type: "complete", IP: details.IP, Data: data, Progress: progress}
}
