package iplookup

import (
	"context"
	"encoding/json"
)

// LiveOptions controls completion-cache reuse and request-local event delivery.
type LiveOptions struct {
	Refresh   bool
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

var livePhases = []string{"topology", "relationships", "names", "asns", "scores", "block", "ptr", "shodan", "transfers"}

func liveComplete(ip string, data []byte) LiveEvent {
	progress := &Progress{Phase: "complete", Completed: len(livePhases), Total: len(livePhases), Tasks: make([]ProgressTask, 0, len(livePhases))}
	for _, phase := range livePhases {
		progress.Tasks = append(progress.Tasks, ProgressTask{Key: phase, Status: "success"})
	}
	return LiveEvent{Type: "complete", IP: ip, Data: data, Progress: progress}
}
