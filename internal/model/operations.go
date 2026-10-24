package model

import "time"

// BackupRun records a database backup attempt.
type BackupRun struct {
	ID          string     `json:"id"`
	Path        string     `json:"path"`
	SizeBytes   int64      `json:"sizeBytes"`
	Status      string     `json:"status"`
	Error       string     `json:"error"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt"`
}

// AuditEvent records a privileged mutation.
type AuditEvent struct {
	ID          string    `json:"id"`
	ActorUserID *string   `json:"actorUserId"`
	Action      string    `json:"action"`
	TargetType  string    `json:"targetType"`
	TargetID    string    `json:"targetId"`
	Detail      string    `json:"detail"`
	CreatedAt   time.Time `json:"createdAt"`
}

// OutboxJob is a durable external synchronization operation.
type OutboxJob struct {
	ClaimGeneration int64     `json:"-"`
	ID              string    `json:"id"`
	Kind            string    `json:"kind"`
	Payload         string    `json:"payload"`
	Status          string    `json:"status"`
	Attempts        int       `json:"attempts"`
	AvailableAt     time.Time `json:"availableAt"`
	LastError       string    `json:"lastError"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// Setting is a safe administrative view. Encrypted values are never returned.
type Setting struct {
	Key        string    `json:"key"`
	Value      string    `json:"value"`
	Encrypted  bool      `json:"encrypted"`
	Configured bool      `json:"configured"`
	Category   string    `json:"category"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
