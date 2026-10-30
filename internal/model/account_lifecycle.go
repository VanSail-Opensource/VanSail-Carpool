package model

import "time"

// AccountPurge is a generation-fenced cleanup target, never a public profile.
type AccountPurge struct {
	UserID        string
	Generation    int64
	Cutoff        time.Time
	RemoteEmbyID  string
	EmbyUncertain bool
	Busy          bool
}
