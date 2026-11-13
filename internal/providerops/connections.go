package providerops

import "time"

// ConnectionScanInput contains only durable scan metadata; raw IPs are excluded.
type ConnectionScanInput struct {
	ActorUserID            string
	ActorAccountGeneration int64
	UserID                 string
	IdempotencyKey         string
	RequestFingerprint     string
	ProviderJobID          string
	ExpiresAt              time.Time
}

// ConnectionScan is provider job progress without connection result data.
type ConnectionScan struct {
	ID                 string
	UserID             string
	IdempotencyKey     string
	RequestFingerprint string
	ProviderJobID      string
	Status             Status
	ProgressPercent    float64
	ErrorCode          string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CompletedAt        *time.Time
	ExpiresAt          time.Time
}

// ConnectionScanUpdate advances provider job metadata monotonically.
type ConnectionScanUpdate struct {
	Status          Status
	ProviderJobID   string
	ProgressPercent float64
	ErrorCode       string
}
