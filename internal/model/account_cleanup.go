package model

import "time"

// AccountCleanupHealth fences all cleanup evidence to a verified numeric bot.
type AccountCleanupHealth struct {
	BotID, Epoch int64
	Verified     bool
	VerifiedAt   *time.Time
	PausedUntil  *time.Time
}

// AccountCleanupProbe is a durable nonce, committed before queued Bot API work.
type AccountCleanupProbe struct {
	UserID                                      string
	TelegramID, AccountGeneration, BotID, Epoch int64
	Nonce                                       string
	StartedAt                                   time.Time
	Final, PriorPrivateReachable                bool
}

// AccountCleanupObservation contains classified responses, never raw messages.
// The caller maps successful getChat to private only for the exact private ID.
type AccountCleanupObservation struct{ SendAction, GetChat string }

// AccountCleanupCompletion describes a nonce-CAS outcome, not account existence.
type AccountCleanupCompletion struct {
	Accepted, Admitted bool
	Generation         int64
	Reason             string
}
