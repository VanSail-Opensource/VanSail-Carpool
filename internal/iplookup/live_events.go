package iplookup

import "encoding/json"

// LiveOptions controls completion-cache reuse and request-local event delivery.
type LiveOptions struct {
	Refresh bool
	Emit    func(LiveEvent) error
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

var livePhases = []string{"topology", "relationships", "names", "asns", "scores", "block", "ptr", "shodan"}

func liveComplete(ip string, data []byte) LiveEvent {
	return LiveEvent{Type: "complete", IP: ip, Data: data, Progress: &Progress{Phase: "complete", Completed: 8, Total: 8}}
}
